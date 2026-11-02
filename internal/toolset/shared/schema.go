package shared

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"
)

type Schema = map[string]any

func Object(properties Schema, required ...string) Schema {
	s := Schema{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}
func StringValue(min, max int) Schema {
	return Schema{"type": "string", "minLength": min, "maxLength": max}
}
func Enum(values ...string) Schema { return Schema{"type": "string", "enum": values} }
func Integer(min, max int) Schema  { return Schema{"type": "integer", "minimum": min, "maximum": max} }
func Array(items Schema, min, max int) Schema {
	return Schema{"type": "array", "items": items, "minItems": min, "maxItems": max}
}
func Raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func NewOperation(id, method, path string, body Schema, fields ...string) Operation {
	return Operation{id, method, "/api/v2" + path, body, fields}
}

func Empty() Schema { return Object(Schema{}) }
func Page() Schema {
	return Object(Schema{"page": Integer(1, 100000), "pageSize": Integer(1, 500)}, "page", "pageSize")
}
func PageSearch() Schema {
	return Object(Schema{"page": Integer(1, 100000), "pageSize": Integer(1, 500), "name": StringValue(0, 256), "order": Enum("null", "ascending", "descending"), "orderBy": Enum("created_at")}, "page", "pageSize", "order", "orderBy")
}
func Domain() Schema {
	return Schema{"type": "string", "minLength": 1, "maxLength": 253, "pattern": "^[A-Za-z0-9*][A-Za-z0-9.*-]*$"}
}

// Tool/API schemas are a deliberately small JSON Schema subset. Core repeats
// authoritative validation; this prevents invalid types from reaching handlers.
func ValidValue(s Schema, v any) bool {
	if values, ok := s["enum"]; ok {
		encoded, _ := json.Marshal(values)
		var list []any
		_ = json.Unmarshal(encoded, &list)
		matched := false
		for _, x := range list {
			if fmt.Sprint(x) == fmt.Sprint(v) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return false
		}
		props, _ := s["properties"].(map[string]any)
		encoded, _ := json.Marshal(s["required"])
		var required []string
		_ = json.Unmarshal(encoded, &required)
		for _, name := range required {
			if _, ok := obj[name]; !ok {
				return false
			}
		}
		for key, value := range obj {
			child, ok := props[key].(map[string]any)
			if !ok || !ValidValue(child, value) {
				return false
			}
		}
	case "string":
		value, ok := v.(string)
		if !ok {
			return false
		}
		n := float64(utf8.RuneCountInString(value))
		if !bounds(s, n, "minLength", "maxLength") {
			return false
		}
		if pattern, ok := s["pattern"].(string); ok {
			re, err := regexp.Compile(pattern)
			if err != nil || !re.MatchString(value) {
				return false
			}
		}
	case "integer", "number":
		n, ok := v.(float64)
		if !ok || math.IsInf(n, 0) || math.IsNaN(n) || (s["type"] == "integer" && n != math.Trunc(n)) || !bounds(s, n, "minimum", "maximum") {
			return false
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return false
		}
	case "array":
		values, ok := v.([]any)
		if !ok || !bounds(s, float64(len(values)), "minItems", "maxItems") {
			return false
		}
		child, ok := s["items"].(map[string]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if !ValidValue(child, value) {
				return false
			}
		}
	case "null":
		return v == nil
	default:
		return false
	}
	return true
}
func bounds(s Schema, n float64, min, max string) bool {
	number := func(v any) float64 {
		switch x := v.(type) {
		case int:
			return float64(x)
		case float64:
			return x
		}
		return 0
	}
	if v, ok := s[min]; ok && n < number(v) {
		return false
	}
	if v, ok := s[max]; ok && n > number(v) {
		return false
	}
	return true
}

type argumentIssue struct {
	path, reason string
}

var safeArgumentField = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

func invalidField(path, reason string) *ExecutionError {
	issue := &argumentIssue{path: path, reason: reason}
	return &ExecutionError{Code: "MCP_INVALID_ARGUMENTS", Message: "Invalid tool argument " + path + ": " + reason, HTTPStatus: 400, argumentIssue: issue}
}

// ValidationDetails only exposes details produced by this validator. It never
// parses upstream error strings or returns rejected argument values/property names.
func ValidationDetails(failure *ExecutionError) (path, reason string, ok bool) {
	if failure == nil || failure.Code != "MCP_INVALID_ARGUMENTS" || failure.argumentIssue == nil {
		return "", "", false
	}
	issue := failure.argumentIssue
	if failure.Message != "Invalid tool argument "+issue.path+": "+issue.reason {
		return "", "", false
	}
	return issue.path, issue.reason, true
}

func argumentChildPath(path, name string) string {
	if !safeArgumentField.MatchString(name) || len(path)+len(name) > 240 {
		return path
	}
	if path == "arguments" {
		return name
	}
	return path + "." + name
}

// Preserve ValidValue as the source of accept/reject decisions. The diagnostic
// traversal runs only for rejected input, so clearer text cannot broaden schemas.
func ValidateValue(s Schema, v any) error {
	return validateValueAt(s, v, "arguments")
}

func validateValueAt(s Schema, v any, path string) error {
	if ValidValue(s, v) {
		return nil
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok || obj == nil {
			return invalidField(path, "must be an object")
		}
		props, _ := s["properties"].(map[string]any)
		encoded, _ := json.Marshal(s["required"])
		var required []string
		_ = json.Unmarshal(encoded, &required)
		for _, name := range required {
			if _, ok := obj[name]; !ok {
				return invalidField(argumentChildPath(path, name), "is required")
			}
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			if _, ok := props[key]; !ok {
				// Unknown property names themselves can contain secrets.
				return invalidField(path, "contains unsupported fields")
			}
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			child, ok := props[key].(map[string]any)
			if !ok {
				return invalidField(path, "does not match the tool schema")
			}
			if err := validateValueAt(child, obj[key], argumentChildPath(path, key)); err != nil {
				return err
			}
		}
	case "string":
		value, ok := v.(string)
		if !ok {
			return invalidField(path, "must be a string")
		}
		if !bounds(s, float64(utf8.RuneCountInString(value)), "minLength", "maxLength") {
			return invalidField(path, "has an invalid length")
		}
		if _, ok := s["enum"]; ok {
			return invalidField(path, "must be one of the allowed values")
		}
		return invalidField(path, "has an invalid format")
	case "integer", "number":
		n, ok := v.(float64)
		if !ok || math.IsInf(n, 0) || math.IsNaN(n) {
			if s["type"] == "integer" {
				return invalidField(path, "must be an integer")
			}
			return invalidField(path, "must be a number")
		}
		if s["type"] == "integer" && n != math.Trunc(n) {
			return invalidField(path, "must be an integer")
		}
		if !bounds(s, n, "minimum", "maximum") {
			return invalidField(path, "is outside the allowed range")
		}
	case "boolean":
		return invalidField(path, "must be a boolean")
	case "array":
		values, ok := v.([]any)
		if !ok {
			return invalidField(path, "must be an array")
		}
		if !bounds(s, float64(len(values)), "minItems", "maxItems") {
			return invalidField(path, "has an invalid number of items")
		}
		child, ok := s["items"].(map[string]any)
		if !ok {
			return invalidField(path, "does not match the tool schema")
		}
		for i, item := range values {
			if err := validateValueAt(child, item, path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
	case "null":
		return invalidField(path, "must be null")
	}
	return invalidField(path, "does not match the tool schema")
}
