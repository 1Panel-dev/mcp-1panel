package shared

import "strings"

type Category struct {
	ID, Name string
	Order    int
}

func NewTool(c Category, handler Handler, id, description string, readOnly bool, input Schema, ops ...Operation) ToolSpec {
	risk := "write"
	if readOnly {
		risk = "read"
	}
	version := 1
	if id != "list_nodes" {
		version = 2
		input["properties"].(map[string]any)["nodeId"] = Schema{
			"type": "string", "minLength": 1, "maxLength": 20,
			"pattern": "^(local|[1-9][0-9]*)$", "default": "local",
			"description": "Target node ID from list_nodes; local means the primary node. Omitted values default to local.",
		}
		description = strings.ReplaceAll(description, "on the primary node", "on the selected node")
		description += "; nodeId defaults to local (primary node)"
	}
	return ToolSpec{Definition: ToolDefinition{ToolID: id, SchemaVersion: version, Name: id, Description: description, CategoryID: c.ID, CategoryName: c.Name, CategoryOrder: c.Order, InputSchema: Raw(input), ReadOnly: readOnly, Risk: risk, Operations: ops}, Handler: handler}
}
