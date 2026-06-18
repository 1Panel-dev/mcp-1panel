package ssl

import "github.com/modelcontextprotocol/go-sdk/mcp"

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        ListSSLs,
		Description: "list ssls",
	}, listSSLs)
	mcp.AddTool(s, &mcp.Tool{
		Name:        CreateSSL,
		Description: "create ssl",
	}, createSSL)
}
