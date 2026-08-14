package ssl

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	CreateSSL = "create_ssl"
)

func createSSL(ctx context.Context, _ *mcp.CallToolRequest, input CreateSSLInput) (*mcp.CallToolResult, any, error) {
	if err := utils.ValidateDomain(input.Domain); err != nil {
		return utils.ToolError(err)
	}
	if input.Provider == "" {
		return utils.ToolError(errors.New("provider is required"))
	}
	if input.Provider != "dnsAccount" && input.Provider != "http" {
		return utils.ToolError(errors.New("provider must be dnsAccount or http"))
	}

	acmeRes := &types.ListAcmeRes{}
	pageReq := &types.PageRequest{
		Page:     1,
		PageSize: 500,
	}
	result, err := utils.NewPanelClient("POST", "/websites/acme/search", utils.WithPayload(pageReq)).Request(ctx, acmeRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}
	if len(acmeRes.Data.Items) == 0 {
		return utils.ToolError(errors.New("no acme account found"))
	}
	acme := acmeRes.Data.Items[0]

	var dnsAccountID uint
	if input.Provider == "dnsAccount" {
		dnsAccountRes := &types.ListDNSAccountRes{}
		result, err = utils.NewPanelClient("POST", "/websites/dns/search", utils.WithPayload(pageReq)).Request(ctx, dnsAccountRes)
		if err != nil {
			return utils.ToolResult(result, err)
		}
		if len(dnsAccountRes.Data.Items) == 0 {
			return utils.ToolError(errors.New("no dns account found"))
		}
		dnsAccountID = input.DnsAccountID
		if dnsAccountID == 0 {
			if parsedID, ok := utils.ParseUintID(input.DnsAccount); ok {
				dnsAccountID = parsedID
			}
		}
		if dnsAccountID == 0 {
			for _, dnsAccount := range dnsAccountRes.Data.Items {
				if dnsAccount.Name == input.DnsAccount {
					dnsAccountID = dnsAccount.ID
					break
				}
			}
		}
		if dnsAccountID == 0 {
			return utils.ToolError(errors.New("dns account is required and must match an existing DNS account ID or exact name"))
		}
	}

	req := &types.CreateSSLRequest{
		PrimaryDomain: input.Domain,
		Provider:      input.Provider,
		AcmeAccountID: acme.ID,
		DnsAccountID:  dnsAccountID,
		KeyType:       "P256",
	}
	res := &types.Response{}
	result, err = utils.NewPanelClient("POST", "/websites/ssl", utils.WithPayload(req)).Request(ctx, res)
	if result != nil {
		result.StructuredContent = res
	}
	return utils.ToolResult(result, err)
}

type CreateSSLInput struct {
	Domain       string `json:"domain" jsonschema:"domain"`
	Provider     string `json:"provider" jsonschema:"provider support dnsAccount,http"`
	DnsAccount   string `json:"dnsAccount,omitempty" jsonschema:"DNS account exact name or numeric ID"`
	DnsAccountID uint   `json:"dns_account_id,omitempty" jsonschema:"DNS account ID"`
}
