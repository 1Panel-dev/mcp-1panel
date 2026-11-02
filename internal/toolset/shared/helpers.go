package shared

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
)

var safePathPart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

const UnknownWriteMessage = "Operation outcome is unknown; check the target resource in 1Panel before retrying"

func UnknownWriteResult() *ExecutionError {
	return &ExecutionError{Code: "MCP_EXECUTION_FAILED", Message: UnknownWriteMessage, HTTPStatus: 502}
}

// Once a mutation has been submitted, an unreadable reply cannot prove it did
// not commit. Keep explicit authorization/business denials, and never retry.
func WriteAPI(ctx context.Context, transport APITransport, path string, body any) (json.RawMessage, error) {
	data, err := API(ctx, transport, "POST", path, body)
	if err == nil {
		return data, nil
	}
	var execution *ExecutionError
	if errors.As(err, &execution) {
		switch execution.Code {
		case "MCP_SCHEMA_MISMATCH", "MCP_NODE_UNAVAILABLE", "MCP_EXECUTION_FAILED":
			failure := UnknownWriteWithHint(execution.Message)
			if IsUnknownWriteMessage(execution.Message) {
				failure.Message = execution.Message
			}
			failure.InvocationID = execution.InvocationID
			return nil, failure
		default:
			return nil, err
		}
	}
	return nil, UnknownWriteResult()
}

func InvalidArguments() error {
	return &ExecutionError{Code: "MCP_INVALID_ARGUMENTS", Message: "Invalid tool arguments", HTTPStatus: 400}
}
func Unavailable(message string) error {
	return &ExecutionError{Code: "MCP_CONFLICT", Message: message, HTTPStatus: 409}
}

func ContractError(message string) error {
	return &ExecutionError{Code: "MCP_SCHEMA_MISMATCH", Message: message, HTTPStatus: 502}
}
func API(ctx context.Context, transport APITransport, method, path string, body any) (json.RawMessage, error) {
	if transport == nil {
		return nil, ContractError("Tool transport is unavailable")
	}
	data, err := transport.RequestAPI(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxAPIResponseBytes || !json.Valid(data) {
		return nil, ContractError("Invalid API response")
	}
	return data, nil
}

const (
	MaxAPIResponseBytes   = 4 << 20
	MaxQueryResponseBytes = 2 << 20
	MaxQueryListItems     = 10000
)

// New metadata queries have a smaller result budget than the transport's
// general response limit. Oversized results are errors, never silent truncation.
func QueryAPI(ctx context.Context, transport APITransport, method, path string, body any) (json.RawMessage, error) {
	data, err := API(ctx, transport, method, path, body)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxQueryResponseBytes {
		return nil, ContractError("Query response exceeds its limit")
	}
	return data, nil
}
func Decode(data json.RawMessage, target any) error {
	if json.Unmarshal(data, target) != nil {
		return ContractError("Invalid API operation result")
	}
	return nil
}
func StringArg(args map[string]any, key, def string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return def
}
func NumberArg(args map[string]any, key string, def int) int {
	if value, ok := args[key].(float64); ok {
		return int(value)
	}
	return def
}
func PageBody() map[string]any { return map[string]any{"page": 1, "pageSize": 500} }

// ValidPathPart reports whether a value is a single safe operation path segment.
func ValidPathPart(value string) bool { return safePathPart.MatchString(value) }
