package website

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

func RegisterTools(s *mcp.Server, allowed func(string) bool) {
	if allowed(ListWebsites) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        ListWebsites,
			Title:       "List websites",
			Description: "List websites managed by 1Panel, optionally filtered by name.",
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: new(bool),
				IdempotentHint:  true,
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[ListWebsitesInput, *types.ListWebsiteRes](listWebsites))
	}
	if allowed(CreateWebsite) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        CreateWebsite,
			Title:       "Create website",
			Description: "Create a static or reverse-proxy website managed by 1Panel.",
			Annotations: &mcp.ToolAnnotations{
				DestructiveHint: new(bool),
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[CreateWebsiteInput, *types.Response](createWebsite))
	}
}
