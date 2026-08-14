package ssl

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

func RegisterTools(s *mcp.Server, allowed func(string) bool) {
	if allowed(ListSSLs) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        ListSSLs,
			Title:       "List SSL certificates",
			Description: "List website SSL certificates managed by 1Panel.",
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: new(bool),
				IdempotentHint:  true,
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[ListSSLsInput, *types.ListWebsiteSSLRes](listSSLs))
	}
	if allowed(CreateSSL) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        CreateSSL,
			Title:       "Create SSL certificate",
			Description: "Request a website SSL certificate through an existing 1Panel ACME account.",
			Annotations: &mcp.ToolAnnotations{
				DestructiveHint: new(bool),
			},
		}, utils.TypedToolHandler[CreateSSLInput, *types.Response](createSSL))
	}
}
