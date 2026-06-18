package database

import "github.com/modelcontextprotocol/go-sdk/mcp"

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        ListDatabases,
		Description: "list databases by name",
	}, listDatabases)
	mcp.AddTool(s, &mcp.Tool{
		Name:        CreateDatabase,
		Description: "create a database by type name and password",
	}, createDatabase)
}
