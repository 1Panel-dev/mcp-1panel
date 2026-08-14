package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/1Panel-dev/mcp-1panel/utils"
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
	panelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":500,"message":"Panel operation failed"}`)
	}))
	t.Cleanup(panelServer.Close)
	utils.SetHost(panelServer.URL)
	utils.SetAccessToken("panel-token")
	t.Cleanup(func() {
		utils.SetHost("")
		utils.SetAccessToken("")
	})

	mcpServer := newMCPServer(AccessReadOnly)
	mcpServer.AddTool(&mcp.Tool{
		Name:        "test_ping",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false},
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil
	})
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
	if len(result.Tools) != 7 {
		t.Fatalf("tools = %d, want 7", len(result.Tools))
	}
	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "test_ping"})
	if err != nil {
		t.Fatal(err)
	}
	if len(callResult.Content) != 1 {
		t.Fatalf("call content = %#v", callResult.Content)
	}
	text, ok := callResult.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "pong" {
		t.Fatalf("call result = %#v", callResult.Content)
	}
	errorResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_system_info"})
	if err != nil {
		t.Fatal(err)
	}
	if !errorResult.IsError || len(errorResult.Content) != 1 {
		t.Fatalf("Panel business error result = %#v", errorResult)
	}
	errorText, ok := errorResult.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(errorText.Text, "API error (500)") {
		t.Fatalf("Panel business error content = %#v", errorResult.Content)
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
