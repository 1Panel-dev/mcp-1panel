package app

import "github.com/modelcontextprotocol/go-sdk/mcp"

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        InstallMySQL,
		Description: "install mysql, if not set name, default is mysql, if not set version, default is '', if not set root_password, default is '')",
	}, installMySQL)
	mcp.AddTool(s, &mcp.Tool{
		Name:        InstallOpenResty,
		Description: "install openresty, if not set name, default is openresty, if not set http_port, default is 80, if not set https_port, default is 443",
	}, installOpenResty)
	mcp.AddTool(s, &mcp.Tool{
		Name:        ListInstalledApps,
		Description: "list installed apps",
	}, listInstalledApps)
}
