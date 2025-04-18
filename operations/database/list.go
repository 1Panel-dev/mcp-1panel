package database

import (
	"context"
	"errors"
	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListDatabases = "list_databases"
)

var ListDatabasesTool = mcp.NewTool(
	ListDatabases,
	mcp.WithDescription("list databases by name"),
	mcp.WithString("name", mcp.Description("database name"), mcp.DefaultString(""), mcp.Required()),
)

func ListDatabasesHandle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var database string
	if request.Params.Arguments["name"] != nil {
		database = request.Params.Arguments["name"].(string)
	}
	if database == "" {
		return nil, errors.New("database name is required")
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
	client := utils.NewPanelClient("POST", "/databases/search", utils.WithPayload(pageReq))
	return client.Request(databaseListRes)
}
