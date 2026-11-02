package shared

import (
	"strings"
)

// MatchOperation checks the compiled operation boundary before a transport can
// execute a handler's subcall. Parameters are single safe URL path segments.
func MatchOperation(op Operation, method, path string) (map[string]string, bool) {
	if method != op.Method || strings.ContainsAny(path, "?#\\") || strings.Contains(path, "%") {
		return nil, false
	}
	actual, template := strings.Split(path, "/"), strings.Split(op.Path, "/")
	if len(actual) != len(template) {
		return nil, false
	}
	params := map[string]string{}
	for i, part := range template {
		if strings.HasPrefix(part, ":") {
			v := actual[i]
			if len(part) < 2 || !safePathPart.MatchString(v) || v == "." || v == ".." {
				return nil, false
			}
			params[part[1:]] = v
		} else if part != actual[i] {
			return nil, false
		}
	}
	return params, true
}

// ProjectResponse keeps only declared scalar fields and explicitly declared
// descendants. Listing a parent never permits nested secrets to escape.
func ProjectResponse(value any, fields []string) any {
	if list, ok := value.([]any); ok {
		result := make([]any, 0, len(list))
		for _, v := range list {
			result = append(result, ProjectResponse(v, fields))
		}
		return result
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	result := map[string]any{}
	children := map[string][]string{}
	for _, field := range fields {
		parts := strings.SplitN(field, ".", 2)
		if len(parts) == 2 {
			children[parts[0]] = append(children[parts[0]], parts[1])
			continue
		}
		if v, ok := obj[field]; ok && safeResponseLeaf(v) {
			result[field] = v
		}
	}
	for key, paths := range children {
		if v, ok := obj[key]; ok {
			result[key] = ProjectResponse(v, paths)
		}
	}
	return result
}

func safeResponseLeaf(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		return false
	case []any:
		for _, item := range v {
			switch item.(type) {
			case map[string]any, []any:
				return false
			}
		}
	}
	return true
}

// ToolOutputSchema describes the common invocation envelope used by both
// transports. Public node discovery has no invocation audit ID to require.
func ToolOutputSchema(definition ToolDefinition) map[string]any {
	var resultSchema any = map[string]any{}
	if len(definition.OutputSchema) > 0 {
		resultSchema = definition.OutputSchema
	}
	required := []string{"invocationId", "nodeId", "result"}
	if definition.ToolID == "list_nodes" {
		required = []string{"nodeId", "result"}
		resultSchema = map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"items"},
			"properties": map[string]any{"items": map[string]any{
				"type": "array", "maxItems": 10000,
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"id", "name", "status", "isLocal"},
					"properties": map[string]any{
						"id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
						"status": map[string]any{"type": "string"}, "isLocal": map[string]any{"type": "boolean"},
					},
				},
			}},
		}
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": required,
		"properties": map[string]any{
			"invocationId": map[string]any{"type": "string"}, "nodeId": map[string]any{"type": "string"}, "result": resultSchema,
		},
	}
}
