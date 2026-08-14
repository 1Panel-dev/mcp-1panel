package app

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

func RegisterTools(s *mcp.Server, allowed func(string) bool) {
	if allowed(InstallMySQL) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        InstallMySQL,
			Title:       "Install MySQL",
			Description: "Install MySQL from the 1Panel app store. Defaults: name mysql and port 3306.",
			Annotations: &mcp.ToolAnnotations{},
		}, utils.TypedToolHandler[InstallMySQLInput, *types.Response](installMySQL))
	}
	if allowed(InstallOpenResty) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        InstallOpenResty,
			Title:       "Install OpenResty",
			Description: "Install OpenResty from the 1Panel app store. Defaults: name openresty and ports 80/443.",
			Annotations: &mcp.ToolAnnotations{},
		}, utils.TypedToolHandler[InstallOpenRestyInput, *types.Response](installOpenResty))
	}
	if allowed(ListInstalledApps) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        ListInstalledApps,
			Title:       "List installed applications",
			Description: "List applications installed through 1Panel.",
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: new(bool),
				IdempotentHint:  true,
				OpenWorldHint:   new(bool),
			},
		}, utils.TypedToolHandler[ListInstalledAppsInput, *types.AppInstalledListResponse](listInstalledApps))
	}
}
