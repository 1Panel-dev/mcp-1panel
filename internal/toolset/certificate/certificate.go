package certificate

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

// Tools returns this module's tool definitions and handlers.
func Tools() []shared.ToolSpec {
	category := shared.Category{ID: "certificate", Name: "Certificate", Order: 30}
	tools := []shared.ToolSpec{}
	tools = append(tools, shared.NewTool(category, listSSLs, "list_ssls", "List certificate metadata; private keys and certificate contents are excluded", true, shared.Empty(), shared.NewOperation("certificates", "POST", "/websites/ssl/search", shared.Page(), "total", "items.id", "items.primaryDomain", "items.domains", "items.provider", "items.organization", "items.autoRenew", "items.expireDate", "items.startDate", "items.status")))
	tools = append(tools, shared.NewTool(category, createSSL, "create_ssl", "Submit asynchronous certificate issuance for a qualified domain or bare IPv4/IPv6 address using existing accounts; specify account IDs when more than one exists. IP certificates require HTTP validation and a Let's Encrypt ACME account; actual issuance depends on CA policy and reachability. dnsAccount and dns_account_id are mutually exclusive. HTTP cannot issue wildcard certificates. DNS provider credentials are never returned", false,
		shared.Object(shared.Schema{"domain": sslDomain(), "provider": shared.Enum("dnsAccount", "http"), "acme_account_id": shared.Integer(1, 2147483647), "dns_account_id": shared.Integer(1, 2147483647), "dnsAccount": shared.StringValue(1, 128)}, "domain", "provider"),
		shared.NewOperation("acme_accounts", "POST", "/websites/acme/search", shared.Page(), "total", "items.id", "items.email", "items.type"),
		shared.NewOperation("dns_accounts", "POST", "/websites/dns/search", shared.Page(), "total", "items.id", "items.name", "items.type"),
		shared.NewOperation("certificate_create", "POST", "/websites/ssl", shared.Object(shared.Schema{"primaryDomain": sslDomain(), "provider": shared.Enum("dnsAccount", "http"), "acmeAccountId": shared.Integer(1, 2147483647), "dnsAccountId": shared.Integer(0, 2147483647), "keyType": shared.Enum("P256"), "isIp": shared.Schema{"type": "boolean", "enum": []bool{true}}}, "primaryDomain", "provider", "acmeAccountId", "dnsAccountId", "keyType"), "id", "primaryDomain", "provider")))
	tools = append(tools, shared.NewTool(category, getSSLMetadata, "get_ssl_metadata", "Get certificate metadata by its explicit ID on the primary node; private keys, certificate contents and account credentials are excluded", true,
		shared.Object(shared.Schema{"ssl_id": shared.Integer(1, 2147483647)}, "ssl_id"),
		shared.NewOperation("certificate", "GET", "/websites/ssl/:id", shared.Empty(), "id", "primaryDomain", "domains", "type", "provider", "organization", "autoRenew", "expireDate", "startDate", "status", "keyType")))
	return tools
}

// The API embeds the complete certificate model. Decode only the fields that
// may leave the panel, even when a transport has not applied its projection.

func getSSLMetadata(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	id := shared.NumberArg(args, "ssl_id", 0)
	data, err := shared.QueryAPI(ctx, transport, "GET", fmt.Sprintf("/websites/ssl/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var result shared.SSLMetadata
	if err = shared.Decode(data, &result); err != nil {
		return nil, err
	}
	if result.ID == 0 || result.ID != uint(id) {
		return nil, shared.ContractError("Invalid certificate metadata result")
	}
	return json.Marshal(result)
}

func listSSLs(ctx context.Context, transport shared.APITransport, _ map[string]any) (json.RawMessage, error) {
	return shared.API(ctx, transport, "POST", "/websites/ssl/search", shared.PageBody())
}
func createSSL(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	domainName := shared.StringArg(args, "domain", "")
	provider := shared.StringArg(args, "provider", "")
	isIP := net.ParseIP(domainName) != nil
	_, hasDNSID := args["dns_account_id"]
	_, hasDNSName := args["dnsAccount"]
	// Conflicting selectors and incompatible challenges are rejected before even
	// reading an account list. Account names never override an explicit ID.
	if !validSSLDomain(domainName) || (provider == "http" && strings.HasPrefix(domainName, "*.")) ||
		(hasDNSID && hasDNSName) || (provider == "http" && (hasDNSID || hasDNSName)) {
		return nil, shared.InvalidArguments()
	}
	if isIP && provider != "http" {
		return nil, &shared.ExecutionError{Code: "MCP_INVALID_ARGUMENTS", Message: "IP certificates support HTTP validation only", HTTPStatus: 400}
	}
	accounts, err := loadSSLAccounts(ctx, transport, "/websites/acme/search")
	if err != nil {
		return nil, err
	}
	acmeID, err := selectSSLAccount(accounts, int64(shared.NumberArg(args, "acme_account_id", 0)), "", "ACME")
	if err != nil {
		return nil, err
	}
	if isIP {
		for _, account := range accounts {
			if account.ID == acmeID && account.Type != "letsencrypt" {
				return nil, shared.Unavailable("IP certificates require a Let's Encrypt ACME account")
			}
		}
	}
	var dnsID int64
	if provider == "dnsAccount" {
		accounts, err = loadSSLAccounts(ctx, transport, "/websites/dns/search")
		if err != nil {
			return nil, err
		}
		dnsID, err = selectSSLAccount(accounts, int64(shared.NumberArg(args, "dns_account_id", 0)), shared.StringArg(args, "dnsAccount", ""), "DNS")
		if err != nil {
			return nil, err
		}
	}
	body := map[string]any{"primaryDomain": domainName, "provider": provider, "acmeAccountId": acmeID, "dnsAccountId": dnsID, "keyType": "P256"}
	if isIP {
		body["isIp"] = true
	}
	data, err := shared.WriteAPI(ctx, transport, "/websites/ssl", body)
	if err != nil {
		return nil, err
	}
	var result struct {
		ID            int64  `json:"id"`
		PrimaryDomain string `json:"primaryDomain"`
		Provider      string `json:"provider"`
		RequestStatus string `json:"requestStatus"`
	}
	if json.Unmarshal(data, &result) != nil || result.ID <= 0 || result.ID > 2147483647 || result.PrimaryDomain != domainName || result.Provider != provider {
		return nil, shared.UnknownWriteResult()
	}
	// The public create API starts issuance asynchronously. A successful response
	// acknowledges its new record; it does not prove that issuance has completed.
	result.RequestStatus = "accepted"
	return json.Marshal(result)
}

func sslDomain() shared.Schema {
	// The pattern keeps the existing qualified DNS-name constraints and admits
	// bare IPv6 candidates. net.ParseIP validates the IP form before any API call.
	return shared.Schema{"type": "string", "minLength": 1, "maxLength": 253, "pattern": `^((\*\.)?([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z0-9][A-Za-z0-9-]{0,61}[A-Za-z0-9]|([0-9]{1,3}\.){3}[0-9]{1,3}|[0-9A-Fa-f.]*:[0-9A-Fa-f:.]*)$`, "description": "A qualified DNS name (wildcards only with DNS validation) or a bare IPv4/IPv6 address. IP addresses require provider=http; do not include a scheme, port, CIDR, brackets or zone."}
}

func validSSLDomain(name string) bool {
	if !shared.ValidValue(sslDomain(), name) {
		return false
	}
	if net.ParseIP(name) != nil {
		return true
	}
	// Colons are only admitted by the IP candidate branch. Reject malformed IPs
	// and wildcard IP addresses instead of treating them as DNS names.
	if strings.Contains(name, ":") || net.ParseIP(strings.TrimPrefix(name, "*.")) != nil {
		return false
	}
	// A malformed dotted IP must not fall back to the DNS-name branch.
	if strings.IndexFunc(strings.TrimPrefix(name, "*."), func(r rune) bool { return r != '.' && (r < '0' || r > '9') }) == -1 {
		return false
	}
	return true
}

type sslAccount struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func loadSSLAccounts(ctx context.Context, transport shared.APITransport, path string) ([]sslAccount, error) {
	const pageSize = 500
	total := -1
	accounts := []sslAccount{}
	seen := map[int64]bool{}
	for pageNumber := 1; pageNumber <= shared.MaxQueryListItems/pageSize; pageNumber++ {
		data, err := shared.QueryAPI(ctx, transport, "POST", path, map[string]any{"page": pageNumber, "pageSize": pageSize})
		if err != nil {
			return nil, err
		}
		var pageResult struct {
			Total *int            `json:"total"`
			Items json.RawMessage `json:"items"`
		}
		if json.Unmarshal(data, &pageResult) != nil || pageResult.Total == nil || len(pageResult.Items) == 0 || *pageResult.Total < 0 || *pageResult.Total > shared.MaxQueryListItems {
			return nil, shared.ContractError("Invalid certificate account list")
		}
		if total < 0 {
			total = *pageResult.Total
		} else if *pageResult.Total != total {
			return nil, shared.ContractError("Certificate account list changed during selection")
		}
		var items []sslAccount
		if json.Unmarshal(pageResult.Items, &items) != nil {
			return nil, shared.ContractError("Invalid certificate account list")
		}
		expected := min(pageSize, total-len(accounts))
		if len(items) != expected {
			return nil, shared.ContractError("Incomplete certificate account list")
		}
		for _, account := range items {
			if account.ID <= 0 || account.ID > 2147483647 || seen[account.ID] {
				return nil, shared.ContractError("Invalid certificate account list")
			}
			seen[account.ID] = true
			accounts = append(accounts, account)
		}
		if len(accounts) == total {
			return accounts, nil
		}
	}
	return nil, shared.ContractError("Certificate account list exceeds its limit")
}

func selectSSLAccount(accounts []sslAccount, id int64, name, kind string) (int64, error) {
	if id == 0 && name == "" {
		if len(accounts) == 1 {
			return accounts[0].ID, nil
		}
		if len(accounts) == 0 {
			return 0, shared.Unavailable("No " + kind + " account exists")
		}
		return 0, shared.Unavailable("More than one " + kind + " account exists; specify its account ID")
	}
	var selected int64
	for _, account := range accounts {
		if (id != 0 && account.ID == id) || (name != "" && strings.EqualFold(account.Name, name)) {
			if selected != 0 {
				return 0, shared.Unavailable("The account name is ambiguous; specify its account ID")
			}
			selected = account.ID
		}
	}
	if selected == 0 {
		return 0, shared.Unavailable("The requested " + kind + " account does not exist")
	}
	return selected, nil
}
