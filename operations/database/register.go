package database

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

func RegisterTools(s *mcp.Server, allowed func(string) bool) {
	if allowed(ListDatabases) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        ListDatabases,
			Title:       "List databases",
			Description: "List 1Panel databases matching the requested database instance name.",
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: new(bool),
				IdempotentHint:  true,
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[ListDatabasesInput, *types.DatabaseListResponse](listDatabases))
	}
	if allowed(CreateDatabase) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        CreateDatabase,
			Title:       "Create database",
			Description: "Create a MySQL or PostgreSQL database in an existing 1Panel database instance.",
			Annotations: &mcp.ToolAnnotations{
				DestructiveHint: new(bool),
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[CreateDatabaseInput, *types.Response](createDatabase))
	}
}
