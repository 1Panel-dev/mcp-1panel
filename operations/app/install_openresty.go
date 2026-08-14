package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	InstallOpenResty = "install_openresty"
)

func installOpenResty(ctx context.Context, _ *mcp.CallToolRequest, input InstallOpenRestyInput) (*mcp.CallToolResult, any, error) {
	name := input.Name
	if name == "" {
		name = "openresty"
	}

	httpPort, err := utils.NormalizePort(input.HttpPort, 80, "http_port")
	if err != nil {
		return utils.ToolError(err)
	}

	httpsPort, err := utils.NormalizePort(input.HttpsPort, 443, "https_port")
	if err != nil {
		return utils.ToolError(err)
	}
	if httpPort == httpsPort {
		return utils.ToolError(errors.New("http_port and https_port must be different"))
	}

	appRes := &types.AppRes{}
	result, err := utils.NewPanelClient("GET", "/apps/openresty").Request(ctx, appRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}
	version, err := utils.SelectExactVersion("", appRes.Data.Versions)
	if err != nil {
		return utils.ToolError(err)
	}
	appID := appRes.Data.ID
	appDetailURL := fmt.Sprintf("/apps/detail/%d/%s/app", appID, version)
	appDetailRes := &types.AppDetailRes{}
	result, err = utils.NewPanelClient("GET", appDetailURL).Request(ctx, appDetailRes)
	if err != nil {
		return utils.ToolResult(result, err)
	}

	appDetailID := appDetailRes.Data.ID

	req := &types.AppInstallCreate{
		AppDetailID: appDetailID,
		Name:        name,
		Params: map[string]interface{}{
			"PANEL_APP_PORT_HTTP":  httpPort,
			"PANEL_APP_PORT_HTTPS": httpsPort,
		},
	}
	res := &types.Response{}
	result, err = utils.NewPanelClient("POST", "/apps/install", utils.WithPayload(req)).Request(ctx, res)
	if result != nil {
		result.StructuredContent = res
	}
	return utils.ToolResult(result, err)
}

type InstallOpenRestyInput struct {
	Name      string  `json:"name,omitempty" jsonschema:"openresty name"`
	HttpPort  float64 `json:"http_port,omitempty" jsonschema:"openresty http port"`
	HttpsPort float64 `json:"https_port,omitempty" jsonschema:"openresty https port"`
}
