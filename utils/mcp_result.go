package utils

import "github.com/modelcontextprotocol/go-sdk/mcp"

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
