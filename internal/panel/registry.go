package panel

import (
	"context"
	"encoding/json"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

type Operation = shared.Operation
type toolHandler func(context.Context, *Client, string, json.RawMessage, string, string) (InvocationResult, error)
type registeredTool struct {
	definition ToolDefinition
	invoke     toolHandler
}

func LocalCatalog() Catalog {
	result := Catalog{ContractVersion: ContractVersion, Tools: []ToolDefinition{}}
	for _, spec := range toolset.All() {
		result.Tools = append(result.Tools, spec.Definition)
	}
	return result
}
func PolicyHash(tool ToolDefinition) string { return shared.PolicyHash(tool) }
func lookupTool(id string) (registeredTool, bool) {
	spec, ok := toolset.Lookup(id)
	if !ok {
		return registeredTool{}, false
	}
	return registeredTool{definition: spec.Definition, invoke: func(ctx context.Context, c *Client, token string, args json.RawMessage, requestID, policy string) (InvocationResult, error) {
		return invokeManaged(ctx, c, token, spec, args, requestID, policy)
	}}, true
}
