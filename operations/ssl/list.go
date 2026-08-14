package ssl

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	ListSSLs = "list_ssls"
)

func listSSLs(ctx context.Context, _ *mcp.CallToolRequest, input ListSSLsInput) (*mcp.CallToolResult, any, error) {
	req := &types.PageRequest{
		Page:     1,
		PageSize: 500,
	}
	listWebsiteSSLRes := &types.ListWebsiteSSLRes{}
	result, err := utils.NewPanelClient("POST", "/websites/ssl/search", utils.WithPayload(req)).Request(ctx, listWebsiteSSLRes)
	if result != nil {
		result.StructuredContent = listWebsiteSSLRes
	}
	return utils.ToolResult(result, err)
}

type ListSSLsInput struct {
}
