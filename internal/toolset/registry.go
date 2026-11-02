package toolset

import (
	"encoding/json"
	"fmt"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/app"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/certificate"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/database"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/system"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/website"
)

type registry struct {
	tools []shared.ToolSpec
	byID  map[string]int
}

func newRegistry(groups ...[]shared.ToolSpec) (*registry, error) {
	r := &registry{byID: map[string]int{}}
	categories := map[string]shared.Category{}
	for _, group := range groups {
		for _, spec := range group {
			d := spec.Definition
			if d.ToolID == "" || d.Name != d.ToolID || spec.Handler == nil || d.SchemaVersion < 1 || len(d.Operations) == 0 || d.CategoryID == "" || d.CategoryName == "" || d.CategoryOrder < 1 {
				return nil, fmt.Errorf("invalid tool registration %q", d.ToolID)
			}
			if _, duplicate := r.byID[d.ToolID]; duplicate {
				return nil, fmt.Errorf("duplicate tool registration %q", d.ToolID)
			}
			c := shared.Category{ID: d.CategoryID, Name: d.CategoryName, Order: d.CategoryOrder}
			if previous, found := categories[c.ID]; found && previous != c {
				return nil, fmt.Errorf("inconsistent category %q", c.ID)
			}
			categories[c.ID] = c
			var input map[string]any
			if json.Unmarshal(d.InputSchema, &input) != nil || input["type"] != "object" || input["additionalProperties"] != false {
				return nil, fmt.Errorf("invalid input schema for %q", d.ToolID)
			}
			operations := map[string]bool{}
			for _, op := range d.Operations {
				if op.ID == "" || operations[op.ID] || op.Method == "" || op.Path == "" {
					return nil, fmt.Errorf("invalid operation for %q", d.ToolID)
				}
				operations[op.ID] = true
			}
			if _, err := json.Marshal(d); err != nil {
				return nil, fmt.Errorf("invalid definition for %q: %w", d.ToolID, err)
			}
			r.byID[d.ToolID] = len(r.tools)
			r.tools = append(r.tools, cloneSpec(spec))
		}
	}
	return r, nil
}

// All construction and validation happen once, before requests are accepted.
// A failure here is a programming error in the compiled-in tool modules.
var builtin = func() *registry {
	r, err := newRegistry(system.Tools(), website.Tools(), certificate.Tools(), app.Tools(), database.Tools())
	if err != nil {
		panic(err)
	}
	return r
}()

func cloneSpec(spec shared.ToolSpec) shared.ToolSpec {
	encoded, _ := json.Marshal(spec.Definition)
	var definition shared.ToolDefinition
	_ = json.Unmarshal(encoded, &definition)
	return shared.ToolSpec{Definition: definition, Handler: spec.Handler}
}

// All and Lookup return independent values, so callers cannot mutate another
// invocation's schema, operation whitelist or display metadata.
func All() []shared.ToolSpec {
	tools := make([]shared.ToolSpec, 0, len(builtin.tools))
	for _, spec := range builtin.tools {
		tools = append(tools, cloneSpec(spec))
	}
	return tools
}
func Lookup(id string) (shared.ToolSpec, bool) {
	index, ok := builtin.byID[id]
	if !ok {
		return shared.ToolSpec{}, false
	}
	return cloneSpec(builtin.tools[index]), true
}
