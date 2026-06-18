package system

import "github.com/modelcontextprotocol/go-sdk/mcp"

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        GetSystemInfo,
		Description: "show host system information, The unit of diskSize is bytes",
	}, getSystemInfo)
	mcp.AddTool(s, &mcp.Tool{
		Name:        GetDashboardInfo,
		Description: "show dashboard info",
	}, getDashboardInfo)
}
