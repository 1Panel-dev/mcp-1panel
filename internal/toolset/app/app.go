package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

func appInstallName() shared.Schema {
	return shared.Schema{"type": "string", "minLength": 2, "maxLength": 30, "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,28}[A-Za-z0-9]$"}
}

// Tools returns this module's tool definitions and handlers.
func Tools() []shared.ToolSpec {
	category := shared.Category{ID: "app", Name: "Application", Order: 40}
	tools := []shared.ToolSpec{}
	for _, app := range []string{"mysql", "openresty"} {
		nameSchema, versionSchema := appInstallName(), shared.StringValue(1, 128)
		nameSchema["description"] = "Required installation name. Use 2-30 letters, digits, underscores or hyphens, starting and ending with a letter or digit."
		versionSchema["description"] = "Required exact version from list_app_versions for the selected node. The version must still exist in that node's current application catalog when installation is submitted."
		input := shared.Schema{"name": nameSchema, "version": versionSchema}
		inputRequired := []string{"name", "version"}
		params := shared.Schema{"PANEL_APP_PORT_HTTP": shared.Integer(1, 65535)}
		required := []string{"PANEL_APP_PORT_HTTP"}
		if app == "mysql" {
			passwordSchema, portSchema := shared.StringValue(8, 128), shared.Integer(1, 65535)
			passwordSchema["description"] = "Required MySQL root password (8-128 characters). Supply the intended password explicitly; the password is never returned."
			portSchema["description"] = "Required MySQL port as a JSON integer between 1 and 65535, for example 3306 (not a quoted string)."
			input["root_password"], input["port"] = passwordSchema, portSchema
			inputRequired = append(inputRequired, "port", "root_password")
			params["PANEL_DB_ROOT_PASSWORD"] = shared.StringValue(8, 128)
			required = append(required, "PANEL_DB_ROOT_PASSWORD")
		} else {
			httpSchema, httpsSchema := shared.Integer(1, 65535), shared.Integer(1, 65535)
			httpSchema["description"] = "Required HTTP port as a JSON integer between 1 and 65535, for example 8088 (not a quoted string); must differ from the HTTPS port."
			httpsSchema["description"] = "Required HTTPS port as a JSON integer between 1 and 65535, for example 8443 (not a quoted string); must differ from the HTTP port."
			input["http_port"], input["https_port"] = httpSchema, httpsSchema
			inputRequired = append(inputRequired, "http_port", "https_port")
			params["PANEL_APP_PORT_HTTPS"] = shared.Integer(1, 65535)
			params["WEBSITE_DIR"] = websiteDirectorySchema()
			required = append(required, "PANEL_APP_PORT_HTTPS", "WEBSITE_DIR")
		}
		operations := []shared.Operation{
			shared.NewOperation("app", "GET", "/apps/"+app, shared.Empty(), "id", "key", "versions"),
			shared.NewOperation("app_detail", "GET", "/apps/detail/:appId/:version/app", shared.Empty(), "id", "appId", "version"),
		}
		if app == "openresty" {
			operations = append(operations, shared.NewOperation("website_dir", "GET", "/settings/website/dir", shared.Empty(), "websiteDir"))
		}
		operations = append(operations, shared.NewOperation("app_install", "POST", "/apps/install", shared.Object(shared.Schema{"appDetailId": shared.Integer(1, 2147483647), "name": appInstallName(), "params": shared.Object(params, required...)}, "appDetailId", "name", "params"), "id", "name", "version", "status"))
		tools = append(tools, shared.NewTool(category, func(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
			return installApp(ctx, transport, app, args)
		}, "install_"+app, "Submit asynchronous "+app+" app installation on the selected node using an explicitly supplied name, exact available version and ports; no passwords or compose configuration are returned", false, shared.Object(input, inputRequired...), operations...))
	}
	tools = append(tools, shared.NewTool(category, listInstalledApps, "list_installed_apps", "List installed applications without environment variables, passwords or compose content", true, shared.Empty(), shared.NewOperation("installed_apps", "POST", "/apps/installed/search", shared.Page(), "total", "items.id", "items.name", "items.version", "items.status", "items.appName", "items.appKey")))
	tools = append(tools, shared.NewTool(category, getInstalledApp, "get_installed_app", "Read one installed application's recorded status, version and ports on the primary node without live container checks; environment, configuration and error messages are excluded", true, shared.Object(shared.Schema{"app_install_id": shared.Integer(1, 2147483647)}, "app_install_id"), shared.NewOperation("installed_app", "GET", "/apps/installed/info/:appInstallId", shared.Empty(), "id", "name", "version", "status", "HttpPort", "appKey", "appPorts")))
	tools = append(tools, shared.NewTool(category, listAppVersions, "list_app_versions", "List available versions for an exact application key with bounded pagination; versions retain the panel catalog order", true, shared.Object(shared.Schema{"app_key": shared.Schema{"type": "string", "minLength": 1, "maxLength": 128, "pattern": "^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$"}, "page": shared.Integer(1, 100000), "page_size": shared.Integer(1, 100)}, "app_key"), shared.NewOperation("app_versions", "GET", "/apps/:key", shared.Empty(), "id", "key", "name", "versions")))
	return tools
}
func listInstalledApps(ctx context.Context, transport shared.APITransport, _ map[string]any) (json.RawMessage, error) {
	return shared.API(ctx, transport, "POST", "/apps/installed/search", shared.PageBody())
}
func installApp(ctx context.Context, transport shared.APITransport, key string, args map[string]any) (json.RawMessage, error) {
	// Required inputs are validated before the handler. Reject port collisions
	// before even catalog discovery.
	if key == "openresty" && shared.NumberArg(args, "http_port", 0) == shared.NumberArg(args, "https_port", 0) {
		return nil, shared.InvalidArguments()
	}
	data, err := shared.API(ctx, transport, "GET", "/apps/"+key, nil)
	if err != nil {
		return nil, err
	}
	var app struct {
		ID       uint     `json:"id"`
		Key      string   `json:"key"`
		Versions []string `json:"versions"`
	}
	if err = shared.Decode(data, &app); err != nil {
		return nil, err
	}
	if app.ID == 0 || app.ID > 2147483647 || app.Key != key {
		return nil, shared.ContractError("Unexpected application result")
	}
	wanted := args["version"].(string)
	version := ""
	for _, v := range app.Versions {
		if v == wanted {
			version = v
			break
		}
	}
	if version == "" || !shared.ValidPathPart(version) || version == "." || version == ".." {
		return nil, shared.Unavailable("Requested application version is unavailable")
	}
	data, err = shared.API(ctx, transport, "GET", fmt.Sprintf("/apps/detail/%d/%s/app", app.ID, version), nil)
	if err != nil {
		return nil, err
	}
	var detail struct {
		ID      uint   `json:"id"`
		AppID   uint   `json:"appId"`
		Version string `json:"version"`
	}
	if err = shared.Decode(data, &detail); err != nil {
		return nil, err
	}
	if detail.ID == 0 || detail.ID > 2147483647 || detail.AppID != app.ID || detail.Version != version {
		return nil, shared.ContractError("Unexpected application detail result")
	}
	params := map[string]any{}
	if key == "mysql" {
		params["PANEL_APP_PORT_HTTP"] = shared.NumberArg(args, "port", 0)
		params["PANEL_DB_ROOT_PASSWORD"] = args["root_password"].(string)
	} else {
		directory, err := loadWebsiteDirectory(ctx, transport)
		if err != nil {
			return nil, err
		}
		params["WEBSITE_DIR"] = directory
		params["PANEL_APP_PORT_HTTP"] = shared.NumberArg(args, "http_port", 0)
		params["PANEL_APP_PORT_HTTPS"] = shared.NumberArg(args, "https_port", 0)
	}
	name := args["name"].(string)
	data, err = shared.WriteAPI(ctx, transport, "/apps/install", map[string]any{"appDetailId": detail.ID, "name": name, "params": params})
	if err != nil {
		return nil, err
	}
	var installed struct {
		ID            uint   `json:"id"`
		Name          string `json:"name"`
		Version       string `json:"version"`
		Status        string `json:"status"`
		RequestStatus string `json:"requestStatus"`
	}
	if json.Unmarshal(data, &installed) != nil || installed.ID == 0 || installed.ID > 2147483647 || installed.Name != name || installed.Version != version {
		return nil, shared.UnknownWriteResult()
	}
	// The background task can already have advanced or failed before the HTTP
	// response is serialized. Accepted describes submission, never completion.
	switch installed.Status {
	case "Installing", "Running", "DownloadErr", "UpErr", "InstallErr":
	default:
		return nil, shared.UnknownWriteResult()
	}
	installed.RequestStatus = "accepted"
	return json.Marshal(installed)
}

// Only the selected node's current website directory can enter this install.
// It is not a tool input: an empty value would change the panel's directory.
func websiteDirectorySchema() shared.Schema {
	return shared.Schema{"type": "string", "minLength": 1, "maxLength": 4096, "pattern": "^/[^\\x00\\r\\n]*$"}
}

func loadWebsiteDirectory(ctx context.Context, transport shared.APITransport) (string, error) {
	data, err := shared.QueryAPI(ctx, transport, "GET", "/settings/website/dir", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Directory string `json:"websiteDir"`
	}
	if err := shared.Decode(data, &result); err != nil {
		return "", err
	}
	if !path.IsAbs(result.Directory) || len(result.Directory) > 4096 || strings.ContainsAny(result.Directory, "\x00\r\n") {
		return "", shared.ContractError("Invalid website directory result")
	}
	return result.Directory, nil
}

// Query DTOs deliberately exclude raw app metadata, error messages, paths and env.
type installedAppQuery struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Status   string `json:"status"`
	HttpPort int    `json:"HttpPort"`
	AppKey   string `json:"appKey"`
	AppPorts []int  `json:"appPorts"`
}

func getInstalledApp(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	id := shared.NumberArg(args, "app_install_id", 0)
	data, err := shared.QueryAPI(ctx, transport, "GET", fmt.Sprintf("/apps/installed/info/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var app installedAppQuery
	if err = shared.Decode(data, &app); err != nil {
		return nil, err
	}
	if app.ID == 0 || app.Status == "Deleted" {
		return nil, shared.Unavailable("The requested installed application does not exist")
	}
	if app.ID != uint(id) || app.Status == "" || len(app.AppPorts) > shared.MaxQueryListItems {
		return nil, shared.ContractError("Unexpected installed application result")
	}
	if app.AppPorts == nil {
		app.AppPorts = []int{}
	}
	return json.Marshal(app)
}

func listAppVersions(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	key := shared.StringArg(args, "app_key", "")
	// These static routes take precedence over /apps/:key in the panel router.
	// Do not let an application key select another API handler.
	if key == "tags" || key == "checkupdate" {
		return nil, shared.InvalidArguments()
	}
	data, err := shared.QueryAPI(ctx, transport, "GET", "/apps/"+key, nil)
	if err != nil {
		return nil, err
	}
	var app struct {
		ID       uint            `json:"id"`
		Key      string          `json:"key"`
		Name     string          `json:"name"`
		Versions json.RawMessage `json:"versions"`
	}
	if err = shared.Decode(data, &app); err != nil {
		return nil, err
	}
	if app.ID == 0 {
		return nil, shared.Unavailable("The requested application does not exist")
	}
	// The ordinary API has a legacy postgres alias; this tool accepts exact keys.
	if app.Key != key {
		return nil, shared.ContractError("Unexpected application result")
	}
	var versions []string
	if len(app.Versions) == 0 || shared.Decode(app.Versions, &versions) != nil || len(versions) > shared.MaxQueryListItems {
		return nil, shared.ContractError("Invalid application versions result")
	}
	for _, version := range versions {
		if version == "" {
			return nil, shared.ContractError("Invalid application version")
		}
	}
	page, size := shared.NumberArg(args, "page", 1), shared.NumberArg(args, "page_size", 50)
	start := min((page-1)*size, len(versions))
	end := min(start+size, len(versions))
	items := append([]string{}, versions[start:end]...)
	return json.Marshal(struct {
		AppID    uint     `json:"appId"`
		AppKey   string   `json:"appKey"`
		Name     string   `json:"name"`
		Total    int      `json:"total"`
		Page     int      `json:"page"`
		PageSize int      `json:"pageSize"`
		Items    []string `json:"items"`
	}{app.ID, app.Key, app.Name, len(versions), page, size, items})
}
