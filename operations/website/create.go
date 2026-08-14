package website

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	CreateWebsite = "create_website"
)

func createWebsite(ctx context.Context, _ *mcp.CallToolRequest, input CreateWebsiteInput) (*mcp.CallToolResult, any, error) {
	if err := utils.ValidateDomain(input.Domain); err != nil {
		return utils.ToolError(err)
	}
	if input.WebsiteType != "static" && input.WebsiteType != "proxy" {
		return utils.ToolError(errors.New("website_type must be static or proxy"))
	}

	domain := input.Domain
	alias := domain
	var proxyAddress string
	if input.WebsiteType == "proxy" {
		if input.ProxyAddress == "" {
			return utils.ToolError(errors.New("proxy_address is required"))
		}
		normalized, err := utils.ValidateProxyAddress(input.ProxyAddress)
		if err != nil {
			return utils.ToolError(err)
		}
		proxyAddress = normalized
	}

	groupReq := &types.GroupRequest{
		Type: "website",
	}
	groupRes := &types.GroupRes{}
	result, err := utils.NewPanelClient("POST", "/groups/search", utils.WithPayload(groupReq)).Request(ctx, groupRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}
	var groupID uint
	for _, group := range groupRes.Data {
		if group.IsDefault {
			groupID = group.ID
			break
		}
	}

	req := &types.CreateWebsiteRequest{
		Domains: []types.WebsiteDomain{
			{
				Domain: domain,
				Port:   80,
				SSL:    false,
			},
		},
		Alias:          alias,
		Type:           input.WebsiteType,
		WebsiteGroupID: groupID,
		Proxy:          proxyAddress,
		AppType:        "new",
	}
	res := &types.Response{}
	result, err = utils.NewPanelClient("POST", "/websites", utils.WithPayload(req)).Request(ctx, res)
	if result != nil {
		result.StructuredContent = res
	}
	return utils.ToolResult(result, err)
}

type CreateWebsiteInput struct {
	Domain       string `json:"domain" jsonschema:"domain,required"`
	WebsiteType  string `json:"website_type" jsonschema:"website type,only support static and proxy,required"`
	ProxyAddress string `json:"proxy_address,omitempty" jsonschema:"proxy address,only support for proxy website"`
}
