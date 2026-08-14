package database

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	CreateDatabase = "create_database"
)

func createDatabase(ctx context.Context, _ *mcp.CallToolRequest, input CreateDatabaseInput) (*mcp.CallToolResult, any, error) {
	if input.Database == "" {
		return utils.ToolError(errors.New("database name is required"))
	}
	if input.DatabaseType == "" {
		return utils.ToolError(errors.New("database type is required"))
	}
	if input.DatabaseType != "mysql" && input.DatabaseType != "postgresql" {
		return utils.ToolError(errors.New("database type is invalid, support mysql and postgresql"))
	}
	if input.Name == "" {
		return utils.ToolError(errors.New("name is required"))
	}

	password := input.Password
	if password == "" {
		generated, err := utils.GenerateSecureString(24)
		if err != nil {
			return utils.ToolError(errors.New("failed to generate secure database password"))
		}
		password = generated
	}
	encodedPassword := base64.StdEncoding.EncodeToString([]byte(password))

	username := input.Username
	if username == "" {
		username = input.Name
	}

	createReq := &types.CreateDatabaseRequest{
		Database: input.Database,
		Password: encodedPassword,
		Type:     input.DatabaseType,
		Name:     input.Name,
		From:     "local",
		Username: username,
	}
	var createURL string
	if input.DatabaseType == "mysql" {
		createURL = "/databases"
		createReq.Format = "utf8mb4"
		createReq.Permission = "%"
	} else {
		createURL = "/databases/pg"
		createReq.Format = "UTF8"
	}
	res := &types.Response{}
	result, err := utils.NewPanelClient("POST", createURL, utils.WithPayload(createReq)).Request(ctx, res)
	if result != nil {
		result.StructuredContent = res
	}
	return utils.ToolResult(result, err)
}

type CreateDatabaseInput struct {
	DatabaseType string `json:"database_type" jsonschema:"installed database app type, support mysql and postgresql"`
	Database     string `json:"database" jsonschema:"installed database app name"`
	Name         string `json:"name" jsonschema:"database name"`
	Username     string `json:"username,omitempty" jsonschema:"database username"`
	Password     string `json:"password,omitempty" jsonschema:"database password"`
}
