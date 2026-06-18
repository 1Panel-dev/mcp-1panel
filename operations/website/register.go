package website

import "github.com/modelcontextprotocol/go-sdk/mcp"

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        ListWebsites,
		Description: "list websites",
	}, listWebsites)
	mcp.AddTool(s, &mcp.Tool{
		Name:        CreateWebsite,
		Description: "create website",
	}, createWebsite)
}
