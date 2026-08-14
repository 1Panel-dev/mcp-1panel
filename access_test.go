package main

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestParseAccessLevel(t *testing.T) {
	tests := []struct {
		input string
		want  AccessLevel
	}{
		{"", AccessReadOnly},
		{"readonly", AccessReadOnly},
		{" READWRITE ", AccessReadWrite},
		{"Full", AccessFull},
	}
	for _, test := range tests {
		got, err := parseAccessLevel(test.input)
		if err != nil {
			t.Fatalf("parseAccessLevel(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("parseAccessLevel(%q) = %s, want %s", test.input, got, test.want)
		}
	}
	if _, err := parseAccessLevel("admin"); err == nil {
		t.Fatal("parseAccessLevel(admin) succeeded")
	}
	if AccessFull.allowsTool("unclassified_tool") {
		t.Fatal("unclassified tool was allowed")
	}
}

func TestToolAccessAndMetadata(t *testing.T) {
	tests := []struct {
		level AccessLevel
		want  []string
	}{
		{AccessReadOnly, []string{
			"get_system_info", "get_dashboard_info", "list_websites", "list_ssls", "list_installed_apps", "list_databases",
		}},
		{AccessReadWrite, []string{
			"get_system_info", "get_dashboard_info", "list_websites", "list_ssls", "list_installed_apps", "list_databases",
			"create_website", "create_ssl", "create_database",
		}},
		{AccessFull, []string{
			"get_system_info", "get_dashboard_info", "list_websites", "list_ssls", "list_installed_apps", "list_databases",
			"create_website", "create_ssl", "create_database", "install_mysql", "install_openresty",
		}},
	}

	for _, test := range tests {
		t.Run(test.level.String(), func(t *testing.T) {
			tools := listTools(t, test.level)
			got := make([]string, 0, len(tools))
			for _, tool := range tools {
				got = append(got, tool.Name)
				if tool.InputSchema == nil || tool.OutputSchema == nil {
					t.Fatalf("tool %s is missing a schema", tool.Name)
				}
				if tool.Annotations == nil {
					t.Fatalf("tool %s is missing annotations", tool.Name)
				}
				required := minimumToolAccess[tool.Name]
				if (required == AccessReadOnly) != tool.Annotations.ReadOnlyHint {
					t.Fatalf("tool %s has incorrect readOnlyHint", tool.Name)
				}
				if tool.Name == "install_mysql" {
					assertSchemaPropertyOptional(t, tool.InputSchema, "name")
				}
			}
			slices.Sort(got)
			slices.Sort(test.want)
			if !slices.Equal(got, test.want) {
				t.Fatalf("tools = %v, want %v", got, test.want)
			}
		})
	}
}

func assertSchemaPropertyOptional(t *testing.T, schema any, property string) {
	t.Helper()
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(metadata.Required, property) {
		t.Fatalf("schema property %q is unexpectedly required", property)
	}
}

func listTools(t *testing.T, access AccessLevel) []*mcp.Tool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := newMCPServer(access)
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-1panel-test", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return result.Tools
}
