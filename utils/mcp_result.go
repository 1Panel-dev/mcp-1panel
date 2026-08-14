package utils

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ToolResult(result *mcp.CallToolResult, err error) (*mcp.CallToolResult, any, error) {
	if result != nil {
		return result, nil, nil
	}
	return nil, nil, err
}

func ToolError(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: err.Error()},
		},
		IsError: true,
	}, nil, nil
}

// TypedToolHandler adds an output schema to an existing handler without
// changing its business behavior. The MCP SDK serializes and validates Out.
func TypedToolHandler[In, Out any](handler mcp.ToolHandlerFor[In, any]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, request *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		result, _, err := handler(ctx, request, input)
		var zero Out
		if err != nil {
			return result, zero, err
		}
		if result == nil {
			return nil, zero, errors.New("tool returned no result")
		}
		if result.IsError {
			return nil, zero, errors.New(toolErrorText(result))
		}

		output, ok := result.StructuredContent.(Out)
		if !ok {
			return nil, zero, errors.New("tool returned an invalid structured result")
		}
		result.StructuredContent = nil
		return result, output, nil
	}
}

func toolErrorText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && text.Text != "" {
			return text.Text
		}
	}
	return "tool execution failed"
}
