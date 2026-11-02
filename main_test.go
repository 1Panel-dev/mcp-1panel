package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/legacy"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestSecureHTTPHandler(t *testing.T) {
	security := httpSecurityConfig{Token: "correct-token"}
	handler := secureHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), security)

	tests := []struct {
		name   string
		token  string
		header string
		origin string
		want   int
	}{
		{"missing token", "", "", "", http.StatusUnauthorized},
		{"wrong token", "wrong", "Authorization", "", http.StatusUnauthorized},
		{"bearer token", "Bearer correct-token", "Authorization", "", http.StatusNoContent},
		{"legacy token rejected", "correct-token", "X-MCP-Token", "", http.StatusUnauthorized},
		{"loopback origin", "Bearer correct-token", "Authorization", "http://localhost:3000", http.StatusNoContent},
		{"foreign origin", "Bearer correct-token", "Authorization", "https://example.com", http.StatusForbidden},
		{"origin with path", "Bearer correct-token", "Authorization", "http://localhost/path", http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", nil)
			if test.header != "" {
				request.Header.Set(test.header, test.token)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
			if test.want == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != `Bearer realm="mcp-1panel"` {
				t.Fatalf("WWW-Authenticate = %q", response.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestHTTPTransportMatchesOnlyConfiguredEndpoint(t *testing.T) {
	endpoint := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	tests := []struct {
		basePath string
		path     string
		want     int
	}{
		{"/mcp", "/mcp", http.StatusNoContent},
		{"/mcp", "/mcp/", http.StatusNotFound},
		{"/mcp", "/mcp/extra", http.StatusNotFound},
		{"/", "/", http.StatusNoContent},
		{"/", "/extra", http.StatusNotFound},
		{"/mcp/", "/mcp/", http.StatusNoContent},
		{"/mcp/", "/mcp/extra", http.StatusNotFound},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodPost, "http://localhost"+test.path, nil)
		response := httptest.NewRecorder()
		newHTTPTransportHandler(test.basePath, endpoint, httpSecurityConfig{AllowInsecure: true}).ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("basePath %q path %q: status = %d, want %d", test.basePath, test.path, response.Code, test.want)
		}
	}
}

func TestStreamableHTTPEndToEnd(t *testing.T) {
	const token = "test-token"
	var panelCalls atomic.Int32
	panelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/dashboard/base/os" || r.Header.Get("CurrentNode") != "local" {
			t.Errorf("unexpected public API request: %s %s node=%q", r.Method, r.URL.Path, r.Header.Get("CurrentNode"))
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("1Panel-Token") == "" || r.Header.Get("1Panel-Timestamp") == "" || r.Header.Get("Authorization") != "" {
			t.Error("public API call must use its own signed API Key identity")
		}
		if panelCalls.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"code":200,"data":{"os":"linux","password":"RAW_SECRET"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"code":500,"message":"RAW_SECRET Panel operation failed"}`)
	}))
	t.Cleanup(panelServer.Close)
	panelClient, err := legacy.NewClient(legacy.Config{Host: panelServer.URL, APIKey: "panel-token"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(panelClient.Close)

	mcpServer := newMCPServer(AccessReadOnly, panelClient)
	handler := newHTTPTransportHandler(
		"/mcp",
		newStreamableHTTPHandler(mcpServer, time.Minute),
		httpSecurityConfig{Token: token},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	httpClient := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.Header = request.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		clone.Header.Set("Accept-Language", "en-US")
		return http.DefaultTransport.RoundTrip(clone)
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-1panel-http-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 15 {
		t.Fatalf("tools = %d, want 15 read-only tools", len(result.Tools))
	}
	foundSystemInfo := false
	for _, tool := range result.Tools {
		if !AccessReadOnly.allowsTool(tool.Name) {
			t.Fatalf("read-only catalog exposed tool %q", tool.Name)
		}
		foundSystemInfo = foundSystemInfo || tool.Name == "get_system_info"
	}
	if !foundSystemInfo {
		t.Fatal("system information tool is missing from the catalog")
	}
	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_system_info"})
	if err != nil {
		t.Fatal(err)
	}
	if callResult.IsError || len(callResult.Content) != 1 {
		t.Fatalf("call content = %#v", callResult.Content)
	}
	text, ok := callResult.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("call result = %#v", callResult.Content)
	}
	var output struct {
		NodeID string            `json:"nodeId"`
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal([]byte(text.Text), &output); err != nil || output.NodeID != "local" || len(output.Result) != 1 || output.Result["os"] != "linux" {
		t.Fatalf("unexpected or unprojected system information: %s (%v)", text.Text, err)
	}
	errorResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_system_info"})
	if err != nil {
		t.Fatal(err)
	}
	if !errorResult.IsError || len(errorResult.Content) != 1 {
		t.Fatalf("Panel business error result = %#v", errorResult)
	}
	errorText, ok := errorResult.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Panel business error content = %#v", errorResult.Content)
	}
	var failure shared.ExecutionError
	if err := json.Unmarshal([]byte(errorText.Text), &failure); err != nil || failure.Code != "MCP_EXECUTION_FAILED" || failure.Message != shared.SystemInfoFailedMessage {
		t.Fatalf("unexpected or unsafe system information error: %s (%v)", errorText.Text, err)
	}
	if calls := panelCalls.Load(); calls != 2 {
		t.Fatalf("public API calls = %d, want one success and one failed read", calls)
	}
}

func TestStreamableHTTPSessionExpires(t *testing.T) {
	handler := newHTTPTransportHandler(
		"/mcp",
		newStreamableHTTPHandler(newMCPServer(AccessReadOnly), 50*time.Millisecond),
		httpSecurityConfig{AllowInsecure: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-1panel-timeout-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	time.Sleep(250 * time.Millisecond)
	if _, err := session.ListTools(ctx, nil); err == nil {
		t.Fatal("expired MCP session still accepted requests")
	}
}

func TestSecureHTTPHandlerLimitsBody(t *testing.T) {
	var receivedLimitError bool
	handler := secureHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, err := io.ReadAll(request.Body)
		var limitError *http.MaxBytesError
		receivedLimitError = errors.As(err, &limitError)
		w.WriteHeader(http.StatusNoContent)
	}), httpSecurityConfig{AllowInsecure: true})

	request := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", bytes.NewReader(make([]byte, (1<<20)+1)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !receivedLimitError {
		t.Fatal("handler did not receive MaxBytesError")
	}
}

func TestHTTPListenSecurity(t *testing.T) {
	secureRemote := httpSecurityConfig{AllowRemote: true, Token: "token"}
	tests := []struct {
		name     string
		scheme   string
		addr     string
		security httpSecurityConfig
		wantErr  bool
	}{
		{"loopback HTTP", "http", "127.0.0.1:8000", httpSecurityConfig{}, false},
		{"loopback IPv6 HTTP", "http", "[::1]:8000", httpSecurityConfig{}, false},
		{"remote not allowed", "https", "192.0.2.10:8000", httpSecurityConfig{Token: "token"}, true},
		{"remote plaintext", "http", "192.0.2.10:8000", secureRemote, true},
		{"remote HTTPS", "https", "192.0.2.10:8000", secureRemote, false},
		{"remote HTTPS no auth", "https", "192.0.2.10:8000", httpSecurityConfig{AllowRemote: true, AllowInsecure: true}, true},
		{"wildcard HTTPS", "https", ":8000", secureRemote, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateHTTPListenAddr(test.scheme, test.addr, test.security)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestParseHTTPAddr(t *testing.T) {
	scheme, listen, path, display, err := parseHTTPAddr("https://127.0.0.1:8443/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if scheme != "https" || listen != "127.0.0.1:8443" || path != "/mcp" || display != "https://127.0.0.1:8443/mcp" {
		t.Fatalf("unexpected parsed address: %q %q %q %q", scheme, listen, path, display)
	}
	if _, _, _, _, err := parseHTTPAddr("ftp://127.0.0.1:21"); err == nil {
		t.Fatal("FTP address was accepted")
	}
	if _, _, _, _, err := parseHTTPAddr("https://user@127.0.0.1:8443"); err == nil {
		t.Fatal("address credentials were accepted")
	}
}

func TestSSETransportIsUnsupported(t *testing.T) {
	err := runServer(context.Background(), "sse", "http://127.0.0.1:8000", AccessReadOnly, httpSecurityConfig{})
	if err == nil || err.Error() != `unsupported transport "sse"` {
		t.Fatalf("error = %v, want unsupported SSE transport", err)
	}
}

func TestRunHTTPServerStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:              "127.0.0.1:0",
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- runHTTPServer(ctx, server, "", "") }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP server did not stop")
	}
}
