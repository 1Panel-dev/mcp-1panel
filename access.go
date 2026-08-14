package main

import (
	"fmt"
	"strings"

	"github.com/1Panel-dev/mcp-1panel/operations/app"
	"github.com/1Panel-dev/mcp-1panel/operations/database"
	"github.com/1Panel-dev/mcp-1panel/operations/ssl"
	"github.com/1Panel-dev/mcp-1panel/operations/system"
	"github.com/1Panel-dev/mcp-1panel/operations/website"
)

type AccessLevel uint8

const (
	AccessReadOnly AccessLevel = iota + 1
	AccessReadWrite
	AccessFull
)

var minimumToolAccess = map[string]AccessLevel{
	system.GetSystemInfo:    AccessReadOnly,
	system.GetDashboardInfo: AccessReadOnly,
	website.ListWebsites:    AccessReadOnly,
	ssl.ListSSLs:            AccessReadOnly,
	app.ListInstalledApps:   AccessReadOnly,
	database.ListDatabases:  AccessReadOnly,
	website.CreateWebsite:   AccessReadWrite,
	ssl.CreateSSL:           AccessReadWrite,
	database.CreateDatabase: AccessReadWrite,
	app.InstallMySQL:        AccessFull,
	app.InstallOpenResty:    AccessFull,
}

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
