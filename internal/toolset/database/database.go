package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

// Tools returns this module's tool definitions and handlers.
func Tools() []shared.ToolSpec {
	category := shared.Category{ID: "database", Name: "Database", Order: 50}
	tools := []shared.ToolSpec{}
	dbOptions := func() shared.Operation {
		return shared.NewOperation("database_instances", "GET", "/databases/db/list/mysql", shared.Empty(), "id", "type", "from", "database")
	}
	instance := func() shared.Schema { return shared.StringValue(1, 128) }
	tools = append(tools, shared.NewTool(category, listDatabases, "list_databases", "List databases in an explicitly named local MySQL instance", true, shared.Object(shared.Schema{"database": instance()}, "database"), dbOptions(), shared.NewOperation("databases", "POST", "/databases/search", shared.Object(shared.Schema{"page": shared.Integer(1, 100000), "pageSize": shared.Integer(1, 500), "database": instance(), "order": shared.Enum("null"), "orderBy": shared.Enum("createdAt")}, "page", "pageSize", "database", "order", "orderBy"), "total", "items.id", "items.name", "items.database", "items.createdAt", "items.description")))
	createString := func(required bool) shared.Schema {
		schema := shared.Schema{"type": "string"}
		if required {
			schema["minLength"] = 1
		}
		return schema
	}
	dbInput := shared.Schema{"database": createString(true), "name": createString(true), "username": createString(false), "password": createString(false), "permission": createString(true)}
	tools = append(tools, shared.NewTool(category, createDatabase, "create_database", "Create a database in an explicitly named local MySQL instance. Business validation is delegated to the existing 1Panel database API. Access defaults to localhost when omitted. Passwords are never returned", false, shared.Object(dbInput, "database", "name"), dbOptions(), shared.NewOperation("database_create", "POST", "/databases", shared.Object(shared.Schema{"database": createString(true), "name": createString(true), "from": shared.Enum("local"), "format": shared.Enum("utf8mb4"), "username": createString(false), "password": createString(false), "permission": createString(true)}, "database", "name", "from", "format", "username", "password", "permission"), "id", "database", "name", "nodeId")))
	tools = append(tools, shared.NewTool(category, listMysqlInstances, "list_mysql_instances", "List installed local MySQL instances on the primary node with bounded pagination; remote databases, addresses and credentials are excluded", true, shared.Object(shared.Schema{"page": shared.Integer(1, 100000), "page_size": shared.Integer(1, 100)}), shared.NewOperation("mysql_instances", "GET", "/databases/db/list/mysql", shared.Empty(), "id", "type", "from", "database", "version")))
	tools = append(tools, shared.NewTool(category, getMysqlStatus, "get_mysql_status", "Read fixed MySQL connection, thread, traffic and uptime counters from an explicitly named installed local MySQL instance", true, shared.Object(shared.Schema{"database": instance()}, "database"), dbOptions(), shared.NewOperation("mysql_status", "POST", "/databases/status", shared.Object(shared.Schema{"name": instance(), "type": shared.Enum("mysql")}, "name", "type"), "Uptime", "Run", "Threads_connected", "Threads_running", "Threads_cached", "Threads_created", "Connections", "Max_used_connections", "Aborted_clients", "Aborted_connects", "Bytes_received", "Bytes_sent", "Questions", "Open_tables")))
	return tools
}
func listDatabases(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	return databaseOperation(ctx, transport, false, args)
}
func createDatabase(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	return databaseOperation(ctx, transport, true, args)
}
func databaseOperation(ctx context.Context, transport shared.APITransport, create bool, args map[string]any) (json.RawMessage, error) {
	instance := shared.StringArg(args, "database", "")
	if instance == "" {
		return nil, shared.InvalidArguments()
	}
	username, password := shared.StringArg(args, "username", ""), shared.StringArg(args, "password", "")
	permission := shared.StringArg(args, "permission", "localhost")

	data, err := shared.API(ctx, transport, "GET", "/databases/db/list/mysql", nil)
	if err != nil {
		return nil, err
	}
	var databases []struct {
		Database string `json:"database"`
		Type     string `json:"type"`
		From     string `json:"from"`
	}
	if err = shared.Decode(data, &databases); err != nil {
		return nil, err
	}
	found := false
	for _, db := range databases {
		if db.Database == instance && db.Type == "mysql" && db.From == "local" {
			found = true
			break
		}
	}
	if !found {
		return nil, shared.Unavailable("The requested local MySQL instance is unavailable")
	}
	if !create {
		body := shared.PageBody()
		body["database"] = instance
		body["order"] = "null"
		body["orderBy"] = "createdAt"
		return shared.API(ctx, transport, "POST", "/databases/search", body)
	}
	return shared.WriteAPI(ctx, transport, "/databases", map[string]any{"database": instance, "name": shared.StringArg(args, "name", ""), "username": username, "password": base64.StdEncoding.EncodeToString([]byte(password)), "from": "local", "format": "utf8mb4", "permission": permission})
}

// Core/Agent enforce this local-instance boundary before returning data or
// contacting MySQL. Filtering again here prevents a malformed response from
// making remote metadata visible in this tool's list or prerequisite lookup.
type mysqlInstanceQuery struct {
	ID       uint   `json:"id"`
	Type     string `json:"type"`
	From     string `json:"from"`
	Database string `json:"database"`
	Version  string `json:"version"`
}

func localMysqlInstances(ctx context.Context, transport shared.APITransport) ([]mysqlInstanceQuery, error) {
	data, err := shared.QueryAPI(ctx, transport, "GET", "/databases/db/list/mysql", nil)
	if err != nil {
		return nil, err
	}
	var options []*mysqlInstanceQuery
	if err = shared.Decode(data, &options); err != nil {
		return nil, err
	}
	if len(options) > shared.MaxQueryListItems {
		return nil, shared.ContractError("Too many MySQL instances")
	}
	local := []mysqlInstanceQuery{}
	for _, option := range options {
		if option == nil || option.ID == 0 || option.Database == "" || option.Type == "" || option.From == "" {
			return nil, shared.ContractError("Invalid MySQL instance result")
		}
		if option.Type == "mysql" && option.From == "local" {
			local = append(local, *option)
		}
	}
	sort.Slice(local, func(i, j int) bool {
		if local[i].Database == local[j].Database {
			return local[i].ID < local[j].ID
		}
		return local[i].Database < local[j].Database
	})
	return local, nil
}

func listMysqlInstances(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	instances, err := localMysqlInstances(ctx, transport)
	if err != nil {
		return nil, err
	}
	page, size := shared.NumberArg(args, "page", 1), shared.NumberArg(args, "page_size", 50)
	start := min((page-1)*size, len(instances))
	end := min(start+size, len(instances))
	return json.Marshal(struct {
		Total    int                  `json:"total"`
		Page     int                  `json:"page"`
		PageSize int                  `json:"pageSize"`
		Items    []mysqlInstanceQuery `json:"items"`
	}{len(instances), page, size, append([]mysqlInstanceQuery{}, instances[start:end]...)})
}

type mysqlStatusQuery struct {
	Uptime             string `json:"Uptime"`
	Run                string `json:"Run"`
	ThreadsConnected   string `json:"Threads_connected"`
	ThreadsRunning     string `json:"Threads_running"`
	ThreadsCached      string `json:"Threads_cached"`
	ThreadsCreated     string `json:"Threads_created"`
	Connections        string `json:"Connections"`
	MaxUsedConnections string `json:"Max_used_connections"`
	AbortedClients     string `json:"Aborted_clients"`
	AbortedConnects    string `json:"Aborted_connects"`
	BytesReceived      string `json:"Bytes_received"`
	BytesSent          string `json:"Bytes_sent"`
	Questions          string `json:"Questions"`
	OpenTables         string `json:"Open_tables"`
}

func getMysqlStatus(ctx context.Context, transport shared.APITransport, args map[string]any) (json.RawMessage, error) {
	instances, err := localMysqlInstances(ctx, transport)
	if err != nil {
		return nil, err
	}
	name := shared.StringArg(args, "database", "")
	found := false
	for _, instance := range instances {
		if instance.Database == name {
			found = true
			break
		}
	}
	if !found {
		return nil, shared.Unavailable("The requested local MySQL instance is unavailable")
	}
	data, err := shared.QueryAPI(ctx, transport, "POST", "/databases/status", map[string]any{"name": name, "type": "mysql"})
	if err != nil {
		return nil, err
	}
	var status mysqlStatusQuery
	if err = shared.Decode(data, &status); err != nil {
		return nil, err
	}
	if _, err := strconv.ParseUint(status.Uptime, 10, 64); err != nil {
		return nil, shared.ContractError("Invalid MySQL status result")
	}
	return json.Marshal(status)
}
