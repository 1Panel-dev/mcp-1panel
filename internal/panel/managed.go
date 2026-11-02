package panel

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

// One instance exists for one tools/call. No credentials, directory or policy
// are shared across requests, and every API subcall re-enters Core authorization.
type apiBridge struct {
	client                       *Client
	token, requestID, policyHash string
	nodeID                       string
	admissionID                  string
	tool                         ToolDefinition
	lastID                       string
}

func (b *apiBridge) RequestAPI(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	for _, op := range b.tool.Operations {
		params, ok := matchOperation(op, method, "/api/v2"+path)
		if !ok {
			continue
		}
		if payload == nil {
			payload = map[string]any{}
		}
		args, err := json.Marshal(payload)
		if err != nil {
			return nil, invalidArguments()
		}
		var value any
		if json.Unmarshal(args, &value) != nil || !validValue(op.BodySchema, value) {
			return nil, invalidArguments()
		}
		result, err := b.client.Invoke(ctx, b.token, Invocation{AdmissionID: b.admissionID, ToolID: b.tool.ToolID, NodeID: b.nodeID, SchemaVersion: b.tool.SchemaVersion, OperationID: op.ID, PolicyHash: b.policyHash, Arguments: args, PathParams: params, RequestID: b.requestID})
		if err != nil {
			return nil, shared.ReadFailure(err, method, "/api/v2"+path)
		}
		b.lastID = result.InvocationID
		// Defense in depth: never allow an unexpected Core response to expand the
		// executable's declared output fields, including nested objects and arrays.
		var data any
		if json.Unmarshal(result.Result, &data) != nil {
			return nil, contractError("Invalid API operation result")
		}
		return json.Marshal(project(data, op.ResponseFields))
	}
	return nil, &RuntimeError{Code: "MCP_FORBIDDEN", Message: "API operation is not declared by this tool", HTTPStatus: http.StatusForbidden}
}

func matchOperation(op Operation, method, path string) (map[string]string, bool) {
	return shared.MatchOperation(op, method, path)
}
func invalidArguments() error {
	return &RuntimeError{Code: "MCP_INVALID_ARGUMENTS", Message: "Invalid tool arguments", HTTPStatus: 400}
}
func validValue(s map[string]any, value any) bool { return shared.ValidValue(s, value) }

func project(value any, fields []string) any {
	return shared.ProjectResponse(value, fields)
}
func invokeManaged(ctx context.Context, c *Client, token string, spec shared.ToolSpec, arguments json.RawMessage, requestID, policy string) (InvocationResult, error) {
	admission, nodeID, err := admitManaged(ctx, c, token, spec, arguments, requestID, policy)
	if err != nil {
		return InvocationResult{}, err
	}
	completed := false
	defer func() {
		if !completed {
			_ = c.Complete(ctx, token, admission.AdmissionID)
		}
	}()
	bridge := &apiBridge{client: c, token: token, requestID: requestID, policyHash: policy, tool: spec.Definition, nodeID: nodeID, admissionID: admission.AdmissionID}
	result, err := spec.Invoke(ctx, bridge, arguments)
	if err != nil {
		failure := runtimeFailure(err)
		if failure.InvocationID == "" {
			// Complete exactly once, even if the final audit cannot be recorded.
			// Neither completion nor its failure may retry a business operation.
			completed = true
			invocationID, completionErr := c.completeOutcome(ctx, token, admission.AdmissionID, failure.Code)
			if completionErr != nil {
				completionFailure := *runtimeFailure(completionErr)
				completionFailure.Message = "Invocation outcome could not be recorded; do not retry a write automatically"
				return InvocationResult{}, &completionFailure
			}
			audited := *failure
			audited.InvocationID = invocationID
			return InvocationResult{}, &audited
		}
		return InvocationResult{}, err
	}
	return InvocationResult{InvocationID: bridge.lastID, NodeID: nodeID, Result: result}, nil
}

func admitManaged(ctx context.Context, c *Client, token string, spec shared.ToolSpec, arguments json.RawMessage, requestID, policy string) (AdmissionResult, string, error) {
	nodeID, err := spec.TargetNode(arguments)
	if err != nil {
		return AdmissionResult{}, "", err
	}
	admission, err := c.Admit(ctx, token, Admission{ToolID: spec.Definition.ToolID, NodeID: nodeID, SchemaVersion: spec.Definition.SchemaVersion, PolicyHash: policy, Arguments: arguments, RequestID: requestID})
	return admission, nodeID, err
}
