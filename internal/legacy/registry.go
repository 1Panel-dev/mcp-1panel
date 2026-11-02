package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/locale"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTools publishes only canonical schemas and uses the same compiled
// business handlers as managed mode. The callback is checked again on execution.
func RegisterTools(server *mcp.Server, allowed func(string) bool, client *Client) error {
	if server == nil || allowed == nil {
		return errors.New("legacy tool registry requires a server and an access policy")
	}
	for _, spec := range toolset.All() {
		description := spec.Definition.Description
		if spec.Definition.ToolID == "get_installed_app" {
			description = strings.Replace(description, "recorded status, version and ports on the selected node without live container checks", "status, version and ports through the public API (which may refresh the panel's status cache)", 1)
		}
		if spec.Definition.ToolID == "list_mysql_instances" || spec.Definition.ToolID == "get_mysql_status" {
			description = strings.ReplaceAll(description, "installed local MySQL", "recorded local MySQL")
		}
		server.AddTool(&mcp.Tool{
			Name:         spec.Definition.Name,
			Description:  description,
			InputSchema:  spec.Definition.InputSchema,
			OutputSchema: outputSchema(spec.Definition),
			Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: spec.Definition.ReadOnly},
		}, toolHandler(spec, allowed, client))
	}
	defaultLanguage := locale.Environment()
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			header := ""
			if extra := request.GetExtra(); extra != nil {
				header = extra.Header.Get("Accept-Language")
			}
			var meta map[string]any
			if call, ok := request.(*mcp.CallToolRequest); ok && call.Params != nil {
				meta = call.Params.GetMeta()
			}
			ctx = locale.RequestContext(ctx, header, meta, defaultLanguage)
			result, err := next(ctx, method, request)
			if err != nil || method != "tools/list" {
				return result, err
			}
			list, ok := result.(*mcp.ListToolsResult)
			if !ok {
				return result, err
			}
			filtered := *list
			filtered.Tools = make([]*mcp.Tool, 0, len(list.Tools))
			for _, tool := range list.Tools {
				if tool != nil && allowed(tool.Name) {
					filtered.Tools = append(filtered.Tools, tool)
				}
			}
			return &filtered, nil
		}
	})
	return nil
}

// Public business APIs have no MCP invocation audit ID. Do not fabricate one
// or weaken the managed output contract to accommodate independent clients.
func outputSchema(definition shared.ToolDefinition) map[string]any {
	result := shared.ToolOutputSchema(definition)
	result["required"] = []string{"nodeId", "result"}
	delete(result["properties"].(map[string]any), "invocationId")
	return result
}

func toolHandler(spec shared.ToolSpec, allowed func(string) bool, client *Client) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if allowed == nil || !allowed(spec.Definition.ToolID) {
			return toolError(ctx, failure("MCP_FORBIDDEN")), nil
		}
		if client == nil {
			return toolError(ctx, failure("MCP_UNAUTHENTICATED")), nil
		}
		ctx, cancel := context.WithTimeout(ctx, 345*time.Second)
		defer cancel()
		arguments := json.RawMessage(`{}`)
		if request != nil && request.Params != nil && len(request.Params.Arguments) > 0 {
			arguments = request.Params.Arguments
		}
		nodeID, err := spec.TargetNode(arguments)
		if err != nil {
			return toolError(ctx, err), nil
		}
		bridge := &apiBridge{client: client, tool: spec.Definition, nodeID: nodeID}
		data, err := spec.Invoke(ctx, bridge, arguments)
		if err != nil {
			return toolError(ctx, err), nil
		}
		var result any
		if json.Unmarshal(data, &result) != nil {
			return toolError(ctx, failure("MCP_SCHEMA_MISMATCH")), nil
		}
		output := struct {
			NodeID string `json:"nodeId"`
			Result any    `json:"result"`
		}{nodeID, result}
		encoded, err := json.Marshal(output)
		if err != nil {
			return toolError(ctx, failure("MCP_SCHEMA_MISMATCH")), nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: output}, nil
	}
}

type apiBridge struct {
	client       *Client
	tool         shared.ToolDefinition
	nodeID       string
	selectedNode node
}

func (b *apiBridge) RequestAPI(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	for _, op := range b.tool.Operations {
		if _, ok := shared.MatchOperation(op, method, "/api/v2"+path); !ok {
			continue
		}
		if payload == nil {
			payload = map[string]any{}
		}
		arguments, err := json.Marshal(payload)
		if err != nil {
			return nil, failure("MCP_INVALID_ARGUMENTS")
		}
		var value any
		if json.Unmarshal(arguments, &value) != nil || !shared.ValidValue(op.BodySchema, value) {
			return nil, failure("MCP_INVALID_ARGUMENTS")
		}
		if b.tool.ToolID == "list_nodes" {
			if method != "GET" || path != "/core/mcp/runtime/nodes" || b.nodeID != "local" {
				return nil, failure("MCP_FORBIDDEN")
			}
			nodes, err := b.client.nodes(ctx)
			if err != nil {
				return nil, err
			}
			return json.Marshal(struct {
				Items []node `json:"items"`
			}{nodes})
		}
		if b.selectedNode.Name == "" {
			selected, err := b.client.resolveNode(ctx, b.nodeID)
			if err != nil {
				return nil, err
			}
			b.selectedNode = selected
		}
		mutation := op.ID == "website_create" || op.ID == "database_create" || op.ID == "certificate_create" || op.ID == "app_install"
		var body any = payload
		if method == "GET" {
			body = nil
		}
		response, err := b.client.request(ctx, method, apiPrefix+path, body, requestOptions{
			NodeName: b.selectedNode.Name, NodeStatus: b.selectedNode.Status, Mutation: mutation,
			AllowEmpty: op.ID == "website_create" || op.ID == "database_create",
			NullList:   op.ID == "database_instances" || op.ID == "mysql_instances" || op.ID == "website_domains" || op.ID == "website_groups",
		})
		if err != nil {
			return nil, shared.ReadFailure(err, method, apiPrefix+path)
		}
		var result any
		if json.Unmarshal(response, &result) != nil {
			return nil, failure("MCP_SCHEMA_MISMATCH")
		}
		// Public create-database acknowledges success without a resource DTO.
		// Echo only the validated target; never invent a database or audit ID.
		if object, ok := result.(map[string]any); ok && op.ID == "database_create" && len(object) == 0 {
			body := value.(map[string]any)
			object["database"], object["name"] = body["database"], body["name"]
		}
		return json.Marshal(shared.ProjectResponse(result, op.ResponseFields))
	}
	return nil, failure("MCP_FORBIDDEN")
}

func toolError(ctx context.Context, err error) *mcp.CallToolResult {
	var execution *shared.ExecutionError
	code := "MCP_EXECUTION_FAILED"
	if errors.As(err, &execution) {
		code = execution.Code
	}
	safe := failure(code)
	if local, ok := shared.SafeExecutionError(execution); ok {
		safe = local
	}
	safe = locale.Error(ctx, safe)
	encoded, _ := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{safe.Code, safe.Message})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, IsError: true}
}
