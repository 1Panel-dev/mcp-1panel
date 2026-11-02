package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// Keep this byte format identical to Core ToolPolicyHash (policy version 4).
func PolicyHash(tool ToolDefinition) string {
	ops := append([]Operation(nil), tool.Operations...)
	slices.SortFunc(ops, func(a, b Operation) int { return strings.Compare(a.ID, b.ID) })
	for i := range ops {
		ops[i].ResponseFields = append([]string(nil), ops[i].ResponseFields...)
		slices.Sort(ops[i].ResponseFields)
	}
	var input map[string]any
	_ = json.Unmarshal(tool.InputSchema, &input)
	encoded, _ := json.Marshal(struct {
		Version       int            `json:"version"`
		SchemaVersion int            `json:"schemaVersion"`
		InputSchema   map[string]any `json:"inputSchema"`
		Operations    []Operation    `json:"operations"`
	}{ToolPolicyVersion, tool.SchemaVersion, input, ops})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
