package main

import (
	"fmt"
	"strings"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset"
)

type AccessLevel uint8

const (
	AccessReadOnly AccessLevel = iota + 1
	AccessReadWrite
	AccessFull
)

var minimumToolAccess = func() map[string]AccessLevel {
	result := make(map[string]AccessLevel)
	for _, spec := range toolset.All() {
		level := AccessReadWrite
		if spec.Definition.ReadOnly {
			level = AccessReadOnly
		}
		if spec.Definition.ToolID == "install_mysql" || spec.Definition.ToolID == "install_openresty" {
			level = AccessFull
		}
		result[spec.Definition.ToolID] = level
	}
	return result
}()

func parseAccessLevel(value string) (AccessLevel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "readonly":
		return AccessReadOnly, nil
	case "readwrite":
		return AccessReadWrite, nil
	case "full":
		return AccessFull, nil
	default:
		return 0, fmt.Errorf("invalid MCP access level %q: use readonly, readwrite, or full", value)
	}
}

func (level AccessLevel) String() string {
	switch level {
	case AccessReadOnly:
		return "readonly"
	case AccessReadWrite:
		return "readwrite"
	case AccessFull:
		return "full"
	default:
		return "invalid"
	}
}

func (level AccessLevel) allowsTool(name string) bool {
	required, classified := minimumToolAccess[name]
	return classified && level >= required
}
