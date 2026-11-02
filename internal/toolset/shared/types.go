// Package shared provides the common tool contract, validation and API helpers.
// It has no dependency on business modules or either runtime adapter.
package shared

import (
	"context"
	"encoding/json"
)

const ContractVersion = 5

// ToolPolicyVersion changes only when the tool policy digest format changes.
// Transport admission does not change existing tool grants or policy hashes.
const ToolPolicyVersion = 4

type Operation struct {
	ID             string         `json:"id"`
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	BodySchema     map[string]any `json:"bodySchema"`
	ResponseFields []string       `json:"responseFields"`
}
type ToolDefinition struct {
	ToolID        string          `json:"toolId"`
	SchemaVersion int             `json:"schemaVersion"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	CategoryID    string          `json:"categoryId"`
	CategoryName  string          `json:"categoryName"`
	CategoryOrder int             `json:"categoryOrder"`
	InputSchema   json.RawMessage `json:"inputSchema"`
	OutputSchema  json.RawMessage `json:"outputSchema,omitempty"`
	ReadOnly      bool            `json:"readOnly"`
	Risk          string          `json:"risk"`
	Operations    []Operation     `json:"operations"`
	PolicyHash    string          `json:"policyHash,omitempty"`
}

type ExecutionError struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	InvocationID      string `json:"invocationId,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	HTTPStatus        int    `json:"-"`
	argumentIssue     *argumentIssue
}

func (e *ExecutionError) Error() string { return e.Code + ": " + e.Message }

// APITransport is supplied independently for each invocation. Implementations
// own authentication and execution; handlers cannot choose a credential or host.
type APITransport interface {
	RequestAPI(context.Context, string, string, any) (json.RawMessage, error)
}

type Handler func(context.Context, APITransport, map[string]any) (json.RawMessage, error)

// ToolSpec binds the published definition and its implementation in one record.
// Handler is deliberately absent from the serialized catalog.
type ToolSpec struct {
	Definition ToolDefinition
	Handler    Handler `json:"-"`
}

func (s ToolSpec) Invoke(ctx context.Context, transport APITransport, arguments json.RawMessage) (json.RawMessage, error) {
	if transport == nil || s.Handler == nil {
		return nil, ContractError("Tool transport or handler is unavailable")
	}
	args, err := s.decodeArguments(arguments)
	if err != nil {
		return nil, err
	}
	// Node selection belongs to the request-scoped transport, never an API DTO.
	delete(args, "nodeId")
	return s.Handler(ctx, transport, args)
}

// TargetNode validates routing independently so a denied known tool can still
// reach Core for an authorization audit even when its business input is invalid.
// Invoke validates the complete input before any business handler executes.
func (s ToolSpec) TargetNode(arguments json.RawMessage) (string, error) {
	var args map[string]any
	var input Schema
	if json.Unmarshal(arguments, &args) != nil || args == nil || json.Unmarshal(s.Definition.InputSchema, &input) != nil {
		return "", InvalidArguments()
	}
	value, present := args["nodeId"]
	if !present {
		return "local", nil
	}
	properties, _ := input["properties"].(map[string]any)
	selector, ok := properties["nodeId"].(map[string]any)
	if !ok {
		return "", invalidField("nodeId", "is not supported by this tool")
	}
	if err := validateValueAt(selector, value, "nodeId"); err != nil {
		return "", err
	}
	return value.(string), nil
}

func (s ToolSpec) decodeArguments(arguments json.RawMessage) (map[string]any, error) {
	var args map[string]any
	var input Schema
	if json.Unmarshal(arguments, &args) != nil || json.Unmarshal(s.Definition.InputSchema, &input) != nil {
		return nil, InvalidArguments()
	}
	if err := validateValueAt(input, args, "arguments"); err != nil {
		return nil, err
	}
	return args, nil
}
