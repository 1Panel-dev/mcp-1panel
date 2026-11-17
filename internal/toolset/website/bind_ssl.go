package website

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

type bindingHTTPS struct {
	Enable *bool `json:"enable"`
	SSL    struct {
		ID *int `json:"id"`
	} `json:"SSL"`
	HTTPConfig            string   `json:"httpConfig"`
	SSLProtocol           []string `json:"SSLProtocol"`
	HSTS                  *bool    `json:"hsts"`
	HSTSIncludeSubDomains *bool    `json:"hstsIncludeSubDomains"`
	HTTP3                 *bool    `json:"http3"`
}

type bindingResult struct {
	WebsiteID    int    `json:"websiteId"`
	WebsiteSSLID int    `json:"websiteSSLId"`
	Status       string `json:"status"`
}

// This tool uses the ordinary HTTPS editor API. Checks describe the observed
// metadata; they do not lock out other API clients or attest the live TLS peer.
func bindSSL(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	websiteID, sslID := shared.NumberArg(args, "website_id", 0), shared.NumberArg(args, "ssl_id", 0)
	endpoint := fmt.Sprintf("/websites/%d/https", websiteID)
	data, err := shared.QueryAPI(ctx, transport, http.MethodGet, fmt.Sprintf("/websites/%d", websiteID), nil)
	if err != nil {
		return nil, err
	}
	var site struct {
		ID              uint   `json:"id"`
		Type            string `json:"type"`
		Status          string `json:"status"`
		Protocol        string `json:"protocol"`
		WebsiteSSLID    *uint  `json:"webSiteSSLId"`
		ParentWebsiteID *uint  `json:"parentWebsiteID"`
	}
	if err = shared.Decode(data, &site); err != nil {
		return nil, err
	}
	if site.ID != uint(websiteID) || site.WebsiteSSLID == nil || site.ParentWebsiteID == nil {
		return nil, shared.ContractError("Invalid website metadata result")
	}
	if site.Type != "static" || site.Status != "Running" || *site.ParentWebsiteID != 0 ||
		(site.Protocol != "HTTP" && site.Protocol != "HTTPS") || (*site.WebsiteSSLID != 0 && *site.WebsiteSSLID != uint(sslID)) {
		return nil, shared.Unavailable("The website or certificate is not ready for binding")
	}
	data, err = shared.QueryAPI(ctx, transport, http.MethodGet, fmt.Sprintf("/websites/domains/%d", websiteID), nil)
	if err != nil {
		return nil, err
	}
	var domains []websiteDomainMetadata
	if err = shared.Decode(data, &domains); err != nil {
		return nil, err
	}
	if len(domains) == 0 || len(domains) > 100 {
		return nil, shared.ContractError("Invalid website domain result")
	}
	for _, domain := range domains {
		if domain.ID == 0 || domain.WebsiteID != uint(websiteID) || domain.Port < 0 || domain.Port > 65535 || !bindingDNSName(domain.Domain, false) {
			return nil, shared.ContractError("Invalid website domain result")
		}
	}
	state, err := readBindingHTTPS(ctx, transport, endpoint)
	if err != nil {
		return nil, err
	}
	if *state.Enable != (site.Protocol == "HTTPS") || *state.SSL.ID != int(*site.WebsiteSSLID) {
		return nil, shared.Unavailable("The website or certificate is not ready for binding")
	}
	data, err = shared.QueryAPI(ctx, transport, http.MethodGet, fmt.Sprintf("/websites/ssl/%d", sslID), nil)
	if err != nil {
		return nil, err
	}
	var certificate shared.SSLMetadata
	if err = shared.Decode(data, &certificate); err != nil {
		return nil, err
	}
	if certificate.ID != uint(sslID) {
		return nil, shared.ContractError("Invalid certificate metadata result")
	}
	if !bindingCertificate(certificate, domains, time.Now()) {
		return nil, shared.Unavailable("Certificate is not ready for binding")
	}
	result := bindingResult{WebsiteID: websiteID, WebsiteSSLID: sslID, Status: "already_bound"}
	if *state.Enable && *state.SSL.ID == sslID {
		return json.Marshal(result)
	}
	if site.Protocol != "HTTP" || *state.SSL.ID != 0 {
		return nil, shared.Unavailable("The website or certificate is not ready for binding")
	}
	// The old POST is a full HTTPS save, not a certificate-ID patch. First
	// enablement uses an explicit policy and the backend's default cipher list.
	body := map[string]any{
		"websiteId": websiteID, "enable": true, "type": "existed", "websiteSSLId": sslID,
		"httpConfig": "HTTPToHTTPS", "SSLProtocol": []string{"TLSv1.3", "TLSv1.2"}, "algorithm": "",
		"hsts": false, "hstsIncludeSubDomains": false, "http3": false,
	}
	if _, err = shared.WriteAPI(ctx, transport, endpoint, body); err != nil {
		return nil, err
	}
	state, err = readBindingHTTPS(ctx, transport, endpoint)
	if err != nil || !*state.Enable || *state.SSL.ID != sslID || state.HTTPConfig != "HTTPToHTTPS" ||
		len(state.SSLProtocol) != 2 || !slices.Contains(state.SSLProtocol, "TLSv1.2") || !slices.Contains(state.SSLProtocol, "TLSv1.3") ||
		state.HSTS == nil || state.HSTSIncludeSubDomains == nil || state.HTTP3 == nil ||
		*state.HSTS || *state.HSTSIncludeSubDomains || *state.HTTP3 {
		return nil, shared.UnknownWriteResult()
	}
	result.Status = "applied"
	return json.Marshal(result)
}

func readBindingHTTPS(ctx context.Context, transport shared.APITransport, endpoint string) (*bindingHTTPS, error) {
	data, err := shared.QueryAPI(ctx, transport, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	var state bindingHTTPS
	if err = shared.Decode(data, &state); err != nil {
		return nil, err
	}
	if state.Enable == nil || state.SSL.ID == nil || *state.SSL.ID < 0 || *state.SSL.ID > 2147483647 {
		return nil, shared.ContractError("Invalid website HTTPS result")
	}
	return &state, nil
}

func bindingCertificate(certificate shared.SSLMetadata, domains []websiteDomainMetadata, now time.Time) bool {
	start, startErr := time.Parse(time.RFC3339, certificate.StartDate)
	expiry, expiryErr := time.Parse(time.RFC3339, certificate.ExpireDate)
	if certificate.Status != "ready" || startErr != nil || expiryErr != nil || now.Before(start) || !now.Before(expiry) {
		return false
	}
	names := append([]string{certificate.PrimaryDomain}, strings.Split(certificate.Domains, ",")...)
	metadata := x509.Certificate{}
	for _, name := range names {
		name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
		if name == "" {
			continue
		}
		if !bindingDNSName(name, true) {
			return false
		}
		metadata.DNSNames = append(metadata.DNSNames, name)
	}
	if len(metadata.DNSNames) == 0 {
		return false
	}
	for _, domain := range domains {
		host := strings.ToLower(strings.TrimSuffix(domain.Domain, "."))
		if metadata.VerifyHostname(host) != nil {
			return false
		}
	}
	return true
}

// Restrict metadata matching to concrete ASCII/punycode hosts and ordinary
// leftmost certificate wildcards; this is not cryptographic SAN verification.
func bindingDNSName(name string, wildcard bool) bool {
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if wildcard && strings.HasPrefix(name, "*.") {
		name = strings.TrimPrefix(name, "*.")
	}
	if name == "" || len(name) > 253 || net.ParseIP(name) != nil {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}
