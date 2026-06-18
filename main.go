package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/app"
	"github.com/1Panel-dev/mcp-1panel/operations/database"
	"github.com/1Panel-dev/mcp-1panel/operations/ssl"
	"github.com/1Panel-dev/mcp-1panel/operations/system"
	"github.com/1Panel-dev/mcp-1panel/operations/website"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

var (
	Version = utils.Version
)

func setupLogger() (*os.File, error) {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Printf("create log dir error: %v\n", err)
		return nil, err
	}

	logFilePath := filepath.Join(logDir, "mcp-1panel.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("open log file error: %v\n", err)
		return nil, err
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return logFile, nil
}

func newMCPServer() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    "github.com/1Panel-dev/mcp-1panel",
		Version: Version,
	}, nil)
}

func addTools(s *mcp.Server) {
	system.RegisterTools(s)
	website.RegisterTools(s)
	ssl.RegisterTools(s)
	app.RegisterTools(s)
	database.RegisterTools(s)
}

type httpSecurityConfig struct {
	Token          string
	AllowedOrigins []string
	AllowInsecure  bool
	AllowRemote    bool
}

func runServer(transport string, addr string, security httpSecurityConfig) error {
	if err := validateHTTPTransportSecurity(transport, security); err != nil {
		return err
	}

	mcpServer := newMCPServer()
	addTools(mcpServer)

	log.Printf("Starting MCP server with transport=%s addr=%s", transport, addr)

	switch strings.ToLower(transport) {
	case "stdio":
		ctx := context.Background()
		log.Printf("Run Stdio server")
		stdioTransport := &mcp.StdioTransport{}
		if err := mcpServer.Run(ctx, stdioTransport); err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case "sse":
		return serveSSE(addr, mcpServer, security)
	case "streamable", "streamable-http":
		return serveStreamableHTTP(addr, mcpServer, security)
	default:
		return fmt.Errorf("unsupported transport %q", transport)
	}
}

func serveSSE(addr string, server *mcp.Server, security httpSecurityConfig) error {
	handler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return serveHTTPTransport("SSE", addr, handler, security)
}

func serveStreamableHTTP(addr string, server *mcp.Server, security httpSecurityConfig) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return serveHTTPTransport("Streamable HTTP", addr, handler, security)
}

func serveHTTPTransport(label, addr string, handler http.Handler, security httpSecurityConfig) error {
	listenAddr, basePath, displayAddr, err := parseHTTPAddr(addr)
	if err != nil {
		return err
	}
	if err := validateHTTPListenAddr(listenAddr, security.AllowRemote); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(basePath, handler)
	if basePath != "/" && !strings.HasSuffix(basePath, "/") {
		mux.Handle(basePath+"/", handler)
	}

	log.Printf("%s transport listening on %s", label, displayAddr)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           secureHTTPHandler(mux, security),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return srv.ListenAndServe()
}

func validateHTTPTransportSecurity(transport string, security httpSecurityConfig) error {
	switch strings.ToLower(transport) {
	case "stdio":
		return nil
	case "sse", "streamable", "streamable-http":
		if security.AllowInsecure {
			return nil
		}
		if security.Token == "" {
			return fmt.Errorf("HTTP transport requires MCP authentication; set -mcp-token or MCP_AUTH_TOKEN, or explicitly use -allow-insecure-http")
		}
		return nil
	default:
		return nil
	}
}

func validateHTTPListenAddr(listenAddr string, allowRemote bool) error {
	if allowRemote {
		return nil
	}

	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("HTTP transport addr %q must include host and port (e.g. http://127.0.0.1:8000): %w", listenAddr, err)
	}
	if host == "" {
		return fmt.Errorf("HTTP transport refuses wildcard listen address %q without -allow-remote-http", listenAddr)
	}

	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}

	return fmt.Errorf("HTTP transport refuses non-loopback listen address %q without -allow-remote-http", listenAddr)
}

func secureHTTPHandler(next http.Handler, security httpSecurityConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic in HTTP transport handler: %v", recovered)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		if !originAllowed(r.Header.Get("Origin"), security.AllowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		if !security.AllowInsecure && !validMCPToken(r, security.Token) {
			http.Error(w, "missing or invalid MCP authentication token", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

func validMCPToken(r *http.Request, expected string) bool {
	if expected == "" {
		return false
	}

	got := r.Header.Get("X-MCP-Token")
	if got == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			got = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, candidate := range allowed {
		if origin == candidate {
			return true
		}
	}
	if len(allowed) > 0 {
		return false
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func parseHTTPAddr(raw string) (listenAddr, basePath, displayAddr string, err error) {
	if raw == "" {
		return "", "", "", fmt.Errorf("addr must not be empty")
	}

	parsedInput := raw
	if !strings.Contains(parsedInput, "://") {
		parsedInput = "http://" + parsedInput
	}

	u, err := url.Parse(parsedInput)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid addr %q: %w", raw, err)
	}

	host := u.Host
	if host == "" {
		return "", "", "", fmt.Errorf("addr %q must include host and port (e.g. http://localhost:8000)", raw)
	}

	path := u.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	display := fmt.Sprintf("%s://%s%s", defaultScheme(u.Scheme), host, path)
	return host, path, display, nil
}

func defaultScheme(s string) string {
	if s == "" {
		return "http"
	}
	return s
}

func main() {
	var (
		transport         string
		accessToken       string
		host              string
		addr              string
		mcpToken          string
		allowedOriginsRaw string
		allowInsecureHTTP bool
		allowRemoteHTTP   bool
	)
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio, sse, streamable-http)")
	flag.StringVar(&addr, "addr", "http://127.0.0.1:8000", "Base URL (host, port, optional path) for HTTP transports")
	flag.StringVar(&accessToken, "token", "", "1Panel api key")
	flag.StringVar(&host, "host", "", "1Panel host (example:http://127.0.0.1:9999)")
	flag.StringVar(&mcpToken, "mcp-token", "", "MCP HTTP authentication token for sse and streamable-http transports")
	flag.StringVar(&allowedOriginsRaw, "allowed-origins", "", "Comma-separated HTTP Origin allowlist for HTTP transports")
	flag.BoolVar(&allowInsecureHTTP, "allow-insecure-http", false, "Allow unauthenticated HTTP transports; only use for local development")
	flag.BoolVar(&allowRemoteHTTP, "allow-remote-http", false, "Allow HTTP transports to listen on non-loopback addresses; only use behind TLS")
	flag.Parse()

	if accessToken != "" {
		utils.SetAccessToken(accessToken)
	}
	if host != "" {
		utils.SetHost(host)
	}

	if mcpToken == "" {
		mcpToken = os.Getenv("MCP_AUTH_TOKEN")
	}

	security := httpSecurityConfig{
		Token:          mcpToken,
		AllowedOrigins: parseAllowedOrigins(allowedOriginsRaw),
		AllowInsecure:  allowInsecureHTTP,
		AllowRemote:    allowRemoteHTTP,
	}

	if err := runServer(transport, addr, security); err != nil {
		fmt.Printf("server run error: %v\n", err)
		os.Exit(1)
	}
}
