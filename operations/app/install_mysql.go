package app

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	InstallMySQL = "install_mysql"
)

func installMySQL(ctx context.Context, _ *mcp.CallToolRequest, input InstallMySQLInput) (*mcp.CallToolResult, any, error) {
	name := input.Name
	if name == "" {
		name = "mysql"
	}

	appRes := &types.AppRes{}
	result, err := utils.NewPanelClient("GET", "/apps/mysql").Request(appRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}

	version, err := utils.SelectExactVersion(input.Version, appRes.Data.Versions)
	if err != nil {
		return utils.ToolError(err)
	}

	appID := appRes.Data.ID
	appDetailURL := fmt.Sprintf("/apps/detail/%d/%s/app", appID, version)
	appDetailRes := &types.AppDetailRes{}
	result, err = utils.NewPanelClient("GET", appDetailURL).Request(appDetailRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}
	appDetailID := appDetailRes.Data.ID

	port, err := utils.NormalizePort(input.Port, 3306, "port")
	if err != nil {
		return utils.ToolError(err)
	}

	rootPassword := input.RootPassword
	if rootPassword == "" {
		generated, err := utils.GenerateSecureString(20)
		if err != nil {
			return utils.ToolError(fmt.Errorf("failed to generate secure root password"))
		}
		rootPassword = fmt.Sprintf("mysql_%s", generated)
	}

	req := &types.AppInstallCreate{
		AppDetailID: appDetailID,
		Name:        name,
		Params: map[string]interface{}{
			"PANEL_APP_PORT_HTTP":    port,
			"PANEL_DB_ROOT_PASSWORD": rootPassword,
		},
	}
	res := &types.Response{}
	result, err = utils.NewPanelClient("POST", "/apps/install", utils.WithPayload(req)).Request(res)
	if result != nil {
		result.StructuredContent = res
	}
	return utils.ToolResult(result, err)
}

type InstallMySQLInput struct {
	Name         string  `json:"name" jsonschema:"mysql name"`
	Version      string  `json:"version,omitempty" jsonschema:"mysql version, not support latest version"`
	RootPassword string  `json:"root_password,omitempty" jsonschema:"mysql root password"`
	Port         float64 `json:"port,omitempty" jsonschema:"mysql port"`
}
