package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

type TransportFunc func(context.Context, string, string, any) (json.RawMessage, error)

func (f TransportFunc) RequestAPI(c context.Context, m, p string, b any) (json.RawMessage, error) {
	return f(c, m, p, b)
}

func AssertQueryRejectsResourceArguments(t *testing.T, lookup func(string) (shared.ToolSpec, bool), id, key string) {
	t.Helper()
	spec, _ := lookup(id)
	arguments := []string{"{}", "null", "[]", "true", "42"}
	for _, value := range []string{"0", "-1", "2147483648", "1.5", "null", "true", `"42"`, `"../ssl/1"`} {
		arguments = append(arguments, fmt.Sprintf(`{%q:%s}`, key, value))
	}
	for _, extra := range []string{`"nodeId":"remote"`, `"token":"secret"`, `"unexpected":true`} {
		arguments = append(arguments, fmt.Sprintf(`{%q:42,%s}`, key, extra))
	}
	for _, args := range arguments {
		_, err := spec.Invoke(context.Background(), TransportFunc(func(context.Context, string, string, any) (json.RawMessage, error) {
			t.Fatalf("invalid arguments reached transport: %s", args)
			return nil, nil
		}), json.RawMessage(args))
		AssertQueryErrorCode(t, err, "MCP_INVALID_ARGUMENTS")
	}
}

func AssertQueryPropagatesFailure(t *testing.T, lookup func(string) (shared.ToolSpec, bool), id, arguments string) {
	t.Helper()
	spec, _ := lookup(id)
	for _, failure := range []error{
		&shared.ExecutionError{Code: "MCP_FORBIDDEN", Message: "denied", HTTPStatus: 403},
		&shared.ExecutionError{Code: "MCP_RESOURCE_NOT_FOUND", Message: "missing", HTTPStatus: 404},
		context.DeadlineExceeded,
	} {
		calls := 0
		result, err := spec.Invoke(context.Background(), TransportFunc(func(context.Context, string, string, any) (json.RawMessage, error) {
			calls++
			return nil, failure
		}), json.RawMessage(arguments))
		if !errors.Is(err, failure) || calls != 1 || result != nil {
			t.Fatalf("failure did not fail closed: result=%s error=%v calls=%d", result, err, calls)
		}
	}
}

func AssertQueryErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var execution *shared.ExecutionError
	if !errors.As(err, &execution) || execution.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func AssertQueryJSON(t *testing.T, result json.RawMessage, want string) {
	t.Helper()
	var actual, expected any
	if json.Unmarshal(result, &actual) != nil || json.Unmarshal([]byte(want), &expected) != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("query result differs\ngot:  %s\nwant: %s", result, want)
	}
	if bytes.Contains(result, []byte("secret")) || bytes.Contains(result, []byte("certificate-body")) {
		t.Fatal("private data escaped metadata filtering")
	}
}

// Lookup selects a tool from the module under test without importing the registry.
func Lookup(tools []shared.ToolSpec, id string) (shared.ToolSpec, bool) {
	for _, spec := range tools {
		if spec.Definition.ToolID == id {
			return spec, true
		}
	}
	return shared.ToolSpec{}, false
}

func CloneSpec(spec shared.ToolSpec) shared.ToolSpec {
	encoded, _ := json.Marshal(spec.Definition)
	var definition shared.ToolDefinition
	_ = json.Unmarshal(encoded, &definition)
	return shared.ToolSpec{Definition: definition, Handler: spec.Handler}
}

func AssertOperation(t *testing.T, spec shared.ToolSpec, method, path string, payload any) {
	t.Helper()
	if payload == nil {
		payload = map[string]any{}
	}
	var body any
	if json.Unmarshal(shared.Raw(payload), &body) != nil {
		t.Fatal("invalid install operation body")
	}
	for _, op := range spec.Definition.Operations {
		if _, ok := shared.MatchOperation(op, method, "/api/v2"+path); ok {
			if !shared.ValidValue(op.BodySchema, body) {
				t.Fatal("handler produced a body outside its manifest", path)
			}
			return
		}
	}
	t.Fatal("handler called an undeclared API", path)
}

func AssertReadonlyGET(t *testing.T, spec shared.ToolSpec) {
	t.Helper()
	if !spec.Definition.ReadOnly || spec.Definition.Risk != "read" {
		t.Fatal("query was not declared read-only")
	}
	for _, op := range spec.Definition.Operations {
		if op.Method != "GET" || !shared.ValidValue(op.BodySchema, map[string]any{}) || shared.ValidValue(op.BodySchema, map[string]any{"nodeId": "remote"}) {
			t.Fatal("resource query operation must have an empty GET body")
		}
		for _, field := range op.ResponseFields {
			for _, forbidden := range []string{"privatekey", "pem", "password", "secret", "authorization", "httpconfig", "servers", "env", "path", "shell", "acmeaccount", "dnsaccount"} {
				if strings.Contains(strings.ToLower(field), forbidden) {
					t.Fatalf("unsafe field in API projection: %s", field)
				}
			}
		}
	}
}
