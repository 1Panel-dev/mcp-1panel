package database

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	ListDatabases = "list_databases"
)

func listDatabases(ctx context.Context, _ *mcp.CallToolRequest, input ListDatabasesInput) (*mcp.CallToolResult, any, error) {
	database := input.Name
	if database == "" {
		return utils.ToolError(errors.New("database name is required"))
	}
	pageReq := &types.ListDatabaseRequest{
		PageRequest: types.PageRequest{
			Page:     1,
			PageSize: 500,
		},
		Order:    "null",
		OrderBy:  "created_at",
		Database: database,
	}
	databaseListRes := &types.DatabaseListResponse{}
	result, err := utils.NewPanelClient("POST", "/databases/search", utils.WithPayload(pageReq)).Request(ctx, databaseListRes)
	if result != nil {
		result.StructuredContent = databaseListRes
	}
	return utils.ToolResult(result, err)
}

type ListDatabasesInput struct {
	Name string `json:"name" jsonschema:"database name"`
}
