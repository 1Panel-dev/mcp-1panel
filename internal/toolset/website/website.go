package website

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

func websiteProxy(min int) shared.Schema {
	value := shared.StringValue(min, 2048)
	// The panel writes this value as an unquoted Nginx directive parameter.
	// Use common Go/ECMAScript syntax, including literal Unicode separators,
	// rather than property escapes that some JSON Schema clients cannot parse.
	// Encoded URL characters remain allowed; raw directive syntax does not.
	value["pattern"] = "^[^\\x00-\\x20\\x7f-\\x9f\u00a0\u00ad\u061c\u1680\u180e\u2000-\u200f\u2028-\u202f\u205f\u2060-\u206f\u3000\ufeff\ufff9-\ufffb;{}'\"\\\\$#]*$"
	return value
}

func websiteAlias(domain string) string {
	if !strings.Contains(domain, "*") {
		return domain
	}
	// Alias becomes a directory and an Nginx include glob. Keep wildcard
	// domains unchanged while giving their storage path a bounded literal name.
	return fmt.Sprintf("wildcard-%x", sha256.Sum256([]byte(domain)))
}

// Tools returns this module's tool definitions and handlers.
func Tools() []shared.ToolSpec {
	category := shared.Category{ID: "website", Name: "Website", Order: 20}
	tools := []shared.ToolSpec{}
	tools = append(tools, shared.NewTool(category, listWebsites, "list_websites", "List up to 500 websites, optionally filtered by name", true, shared.Object(shared.Schema{"name": shared.StringValue(0, 256)}), shared.NewOperation("websites", "POST", "/websites/search", shared.PageSearch(), "total", "items.id", "items.primaryDomain", "items.type", "items.alias", "items.status", "items.protocol", "items.expireDate", "items.sslExpireDate")))
	port := shared.Integer(1, 65535)
	port["description"] = "HTTP listen port; when omitted, use the target node's installed OpenResty HTTP port"
	tools = append(tools, shared.NewTool(category, createWebsite, "create_website", "Create a static or reverse proxy website in the default website group on the primary node; an omitted port uses that node's installed OpenResty HTTP port", false,
		shared.Object(shared.Schema{"domain": shared.Domain(), "website_type": shared.Enum("static", "proxy"), "proxy_address": websiteProxy(1), "port": port}, "domain", "website_type"),
		shared.NewOperation("openresty_check", "POST", "/apps/installed/check", shared.Object(shared.Schema{"key": shared.Enum("openresty"), "name": shared.Enum("")}, "key", "name"), "isExist", "httpPort"),
		shared.NewOperation("website_groups", "POST", "/groups/search", shared.Object(shared.Schema{"type": shared.Enum("website")}, "type"), "id", "isDefault"),
		shared.NewOperation("website_create", "POST", "/websites", shared.Object(shared.Schema{"type": shared.Enum("static", "proxy"), "alias": shared.StringValue(1, 253), "webSiteGroupID": shared.Integer(1, 2147483647), "proxy": websiteProxy(0), "appType": shared.Enum("new"), "domains": shared.Array(shared.Object(shared.Schema{"domain": shared.Domain(), "port": shared.Integer(1, 65535), "ssl": shared.Schema{"type": "boolean"}}, "domain", "port", "ssl"), 1, 1)}, "type", "alias", "webSiteGroupID", "proxy", "appType", "domains"), "id", "primaryDomain")))
	tools = append(tools, shared.NewTool(category, getWebsite, "get_website", "Get website metadata by its explicit ID on the primary node; configuration, paths and runtime environment are excluded", true,
		shared.Object(shared.Schema{"website_id": shared.Integer(1, 2147483647)}, "website_id"),
		shared.NewOperation("website", "GET", "/websites/:id", shared.Empty(), "id", "primaryDomain", "type", "alias", "status", "protocol", "expireDate", "webSiteGroupId", "webSiteSSLId", "appInstallId", "runtimeID", "parentWebsiteID")))
	tools = append(tools, shared.NewTool(category, listWebsiteDomains, "list_website_domains", "List recorded domains and ports of an explicit website ID on the primary node, sorted by domain ID; port 0 means no explicit port is recorded and does not identify the actual listening port; page defaults to 1 and page_size to 50 (maximum 100)", true,
		shared.Object(shared.Schema{"website_id": shared.Integer(1, 2147483647), "page": shared.Integer(1, 100000), "page_size": shared.Integer(1, 100)}, "website_id"),
		shared.NewOperation("website_domains", "GET", "/websites/domains/:websiteId", shared.Empty(), "id", "websiteId", "domain", "port", "ssl")))
	tools = append(tools, shared.NewTool(category, getWebsiteHTTPS, "get_website_https", "Get HTTPS settings and certificate metadata for an explicit website ID on the primary node; private keys and certificate contents are excluded", true,
		shared.Object(shared.Schema{"website_id": shared.Integer(1, 2147483647)}, "website_id"),
		shared.NewOperation("website_https", "GET", "/websites/:id/https", shared.Empty(), "enable", "SSLProtocol", "hsts", "hstsIncludeSubDomains", "httpsPorts", "httpsPort", "http3", "SSL.id", "SSL.primaryDomain", "SSL.domains", "SSL.type", "SSL.provider", "SSL.organization", "SSL.autoRenew", "SSL.expireDate", "SSL.startDate", "SSL.status", "SSL.keyType")))
	return tools
}

type websiteMetadata struct {
	ID              uint   `json:"id"`
	PrimaryDomain   string `json:"primaryDomain"`
	Type            string `json:"type"`
	Alias           string `json:"alias"`
	Status          string `json:"status"`
	Protocol        string `json:"protocol"`
	ExpireDate      string `json:"expireDate"`
	WebsiteGroupID  uint   `json:"webSiteGroupId"`
	WebsiteSSLID    uint   `json:"webSiteSSLId"`
	AppInstallID    uint   `json:"appInstallId"`
	RuntimeID       uint   `json:"runtimeID"`
	ParentWebsiteID uint   `json:"parentWebsiteID"`
}

func getWebsite(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	id := shared.NumberArg(args, "website_id", 0)
	data, err := shared.QueryAPI(ctx, transport, "GET", fmt.Sprintf("/websites/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var result websiteMetadata
	if err = shared.Decode(data, &result); err != nil {
		return nil, err
	}
	if result.ID == 0 || result.ID != uint(id) {
		return nil, shared.ContractError("Invalid website metadata result")
	}
	return json.Marshal(result)
}

type websiteDomainMetadata struct {
	ID        uint   `json:"id"`
	WebsiteID uint   `json:"websiteId"`
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	SSL       bool   `json:"ssl"`
}

func listWebsiteDomains(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	id := shared.NumberArg(args, "website_id", 0)
	data, err := shared.QueryAPI(ctx, transport, "GET", fmt.Sprintf("/websites/domains/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var result []websiteDomainMetadata
	if err = shared.Decode(data, &result); err != nil {
		return nil, err
	}
	if len(result) > shared.MaxQueryListItems {
		return nil, shared.ContractError("Website domain result exceeds the supported limit")
	}
	for _, item := range result {
		if item.ID == 0 || item.WebsiteID != uint(id) || item.Domain == "" || item.Port < 0 || item.Port > 65535 {
			return nil, shared.ContractError("Invalid website domain result")
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	page, pageSize := shared.NumberArg(args, "page", 1), shared.NumberArg(args, "page_size", 50)
	start := int64(page-1) * int64(pageSize)
	items := []websiteDomainMetadata{}
	if start < int64(len(result)) {
		end := start + int64(pageSize)
		if end > int64(len(result)) {
			end = int64(len(result))
		}
		items = result[start:end]
	}
	return json.Marshal(struct {
		Total    int                     `json:"total"`
		Page     int                     `json:"page"`
		PageSize int                     `json:"pageSize"`
		Items    []websiteDomainMetadata `json:"items"`
	}{Total: len(result), Page: page, PageSize: pageSize, Items: items})
}

type websiteHTTPSMetadata struct {
	Enable                *bool               `json:"enable"`
	SSL                   *shared.SSLMetadata `json:"SSL"`
	SSLProtocol           []string            `json:"SSLProtocol"`
	Hsts                  bool                `json:"hsts"`
	HstsIncludeSubDomains bool                `json:"hstsIncludeSubDomains"`
	HTTPSPorts            []int               `json:"httpsPorts"`
	HTTPSPort             string              `json:"httpsPort"`
	HTTP3                 bool                `json:"http3"`
}

func getWebsiteHTTPS(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	id := shared.NumberArg(args, "website_id", 0)
	data, err := shared.QueryAPI(ctx, transport, "GET", fmt.Sprintf("/websites/%d/https", id), nil)
	if err != nil {
		return nil, err
	}
	var result websiteHTTPSMetadata
	if err = shared.Decode(data, &result); err != nil {
		return nil, err
	}
	if result.Enable == nil || (*result.Enable && (result.SSL == nil || result.SSL.ID == 0)) {
		return nil, shared.ContractError("Invalid website HTTPS result")
	}
	if result.SSL != nil && result.SSL.ID == 0 {
		result.SSL = nil
	}
	return json.Marshal(result)
}

func listWebsites(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	body := shared.PageBody()
	body["name"] = shared.StringArg(args, "name", "")
	body["order"] = "null"
	body["orderBy"] = "created_at"
	return shared.API(ctx, transport, "POST", "/websites/search", body)
}
func createWebsite(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	kind := shared.StringArg(args, "website_type", "")
	proxy := shared.StringArg(args, "proxy_address", "")
	if kind == "proxy" {
		if !shared.ValidValue(websiteProxy(1), proxy) {
			return nil, shared.InvalidArguments()
		}
		if strings.IndexFunc(proxy, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
		}) >= 0 {
			return nil, shared.InvalidArguments()
		}
		u, err := url.Parse(proxy)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
			return nil, shared.InvalidArguments()
		}
	} else if proxy != "" {
		return nil, shared.InvalidArguments()
	}
	port := shared.NumberArg(args, "port", 0)
	if _, specified := args["port"]; !specified {
		// Old public APIs may persist port 0 literally. Resolve the selected
		// node's real default and send an explicit port in both startup modes.
		data, err := shared.API(ctx, transport, "POST", "/apps/installed/check", map[string]any{"key": "openresty", "name": ""})
		if err != nil {
			return nil, err
		}
		var installed struct {
			Exists   *bool `json:"isExist"`
			HTTPPort *int  `json:"httpPort"`
		}
		if err = shared.Decode(data, &installed); err != nil {
			return nil, err
		}
		if installed.Exists == nil {
			return nil, shared.ContractError("Invalid OpenResty installation result")
		}
		if !*installed.Exists {
			return nil, shared.Unavailable("OpenResty is not installed on the selected node")
		}
		if installed.HTTPPort == nil || *installed.HTTPPort < 1 || *installed.HTTPPort > 65535 {
			return nil, shared.ContractError("Invalid OpenResty HTTP port")
		}
		port = *installed.HTTPPort
	}
	data, err := shared.API(ctx, transport, "POST", "/groups/search", map[string]any{"type": "website"})
	if err != nil {
		return nil, err
	}
	var groups []struct {
		ID      uint `json:"id"`
		Default bool `json:"isDefault"`
	}
	if err = shared.Decode(data, &groups); err != nil {
		return nil, err
	}
	var groupID uint
	for _, g := range groups {
		if g.Default {
			groupID = g.ID
			break
		}
	}
	if groupID == 0 {
		return nil, shared.Unavailable("No default website group exists")
	}
	domain := shared.StringArg(args, "domain", "")
	_, err = shared.WriteAPI(ctx, transport, "/websites", map[string]any{"type": kind, "alias": websiteAlias(domain), "webSiteGroupID": groupID, "proxy": proxy, "appType": "new", "domains": []any{map[string]any{"domain": domain, "port": port, "ssl": false}}})
	if err != nil {
		return nil, err
	}
	return shared.Raw(map[string]any{"status": "completed", "domain": domain}), nil
}
