package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"encoding/json"
	"github.com/1Panel-dev/mcp-1panel/internal/legacy"
	"github.com/1Panel-dev/mcp-1panel/internal/locale"
	"github.com/1Panel-dev/mcp-1panel/internal/panel"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

var (
	Version = utils.Version
)

const streamableSessionTimeout = 30 * time.Minute

func newMCPServer(access AccessLevel, clients ...*legacy.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "github.com/1Panel-dev/mcp-1panel",
		Version: Version,
	}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{},
	})
	addTools(server, access, clients...)
	return server
}

func addTools(s *mcp.Server, access AccessLevel, clients ...*legacy.Client) {
	var client *legacy.Client
	if len(clients) > 0 {
		client = clients[0]
	}
	if err := legacy.RegisterTools(s, access.allowsTool, client); err != nil {
		panic(err)
	}
}

type httpSecurityConfig struct {
	Token          string
	AllowedOrigins []string
	AllowInsecure  bool
	AllowRemote    bool
	TLSDir         string
	TLSHosts       []string
}

func runServer(ctx context.Context, transport, addr string, access AccessLevel, security httpSecurityConfig, clients ...*legacy.Client) error {
	if err := validateHTTPTransportSecurity(transport, security); err != nil {
		return err
	}

	mcpServer := newMCPServer(access, clients...)

	log.Printf("starting MCP server transport=%s access=%s", transport, access)

	switch strings.ToLower(transport) {
	case "stdio":
		if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case "streamable", "streamable-http":
		return serveStreamableHTTP(ctx, addr, mcpServer, security)
	default:
		return fmt.Errorf("unsupported transport %q", transport)
	}
}

func serveStreamableHTTP(ctx context.Context, addr string, server *mcp.Server, security httpSecurityConfig) error {
	handler := newStreamableHTTPHandler(server, streamableSessionTimeout)
	return serveHTTPTransport(ctx, "Streamable HTTP", addr, handler, security)
}

func newStreamableHTTPHandler(server *mcp.Server, sessionTimeout time.Duration) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		SessionTimeout: sessionTimeout,
	})
}

func serveHTTPTransport(ctx context.Context, label, addr string, handler http.Handler, security httpSecurityConfig) error {
	scheme, listenAddr, basePath, displayAddr, err := parseHTTPAddr(addr)
	if err != nil {
		return err
	}
	if err := validateHTTPListenAddr(scheme, listenAddr, security); err != nil {
		return err
	}

	log.Printf("%s transport listening on %s", label, displayAddr)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           newHTTPTransportHandler(basePath, handler, security),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	var certFile, keyFile string
	if scheme == "https" {
		material, err := ensureTLSMaterial(security.TLSDir, security.TLSHosts, listenAddr, time.Now())
		if err != nil {
			return fmt.Errorf("prepare TLS: %w", err)
		}
		certFile, keyFile = material.serverCert, material.serverKey
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		fmt.Fprintf(os.Stderr, "MCP CA certificate: %s\n", material.caCert)
		fmt.Fprintf(os.Stderr, "MCP CA SHA256 fingerprint: %s\n", material.fingerprint)
		fmt.Fprintf(os.Stderr, "MCP server certificate SANs: %s\n", strings.Join(material.hosts, ", "))
	}
	return runHTTPServer(ctx, srv, certFile, keyFile)
}

func newHTTPTransportHandler(basePath string, handler http.Handler, security httpSecurityConfig) http.Handler {
	secured := secureHTTPHandler(handler, security)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != basePath {
			http.NotFound(w, r)
			return
		}
		secured.ServeHTTP(w, r)
	})
}

func runHTTPServer(ctx context.Context, server *http.Server, certFile, keyFile string) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	errCh := make(chan error, 1)
	go func() {
		if certFile != "" {
			errCh <- server.ServeTLS(listener, certFile, keyFile)
			return
		}
		errCh <- server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func validateHTTPTransportSecurity(transport string, security httpSecurityConfig) error {
	switch strings.ToLower(transport) {
	case "stdio":
		return nil
	case "streamable", "streamable-http":
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

func validateHTTPListenAddr(scheme, listenAddr string, security httpSecurityConfig) error {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("HTTP transport addr %q must include host and port (e.g. http://127.0.0.1:8000): %w", listenAddr, err)
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	if !security.AllowRemote {
		return fmt.Errorf("HTTP transport refuses non-loopback listen address %q without -allow-remote-http", listenAddr)
	}
	if scheme != "https" {
		return fmt.Errorf("HTTP transport refuses non-loopback listen address %q without HTTPS", listenAddr)
	}
	if security.AllowInsecure || security.Token == "" {
		return errors.New("remote HTTPS transport requires MCP authentication")
	}
	return nil
}

func secureHTTPHandler(next http.Handler, security httpSecurityConfig) http.Handler {
	defaultLanguage := locale.Environment()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(locale.RequestContext(r.Context(), r.Header.Get("Accept-Language"), nil, defaultLanguage))
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic in HTTP transport handler")
				http.Error(w, locale.Text(r.Context(), http.StatusText(http.StatusInternalServerError)), http.StatusInternalServerError)
			}
		}()

		if !originAllowed(r.Header.Get("Origin"), security.AllowedOrigins) {
			http.Error(w, locale.Text(r.Context(), "origin not allowed"), http.StatusForbidden)
			return
		}

		if !security.AllowInsecure && !validMCPToken(r, security.Token) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-1panel"`)
			http.Error(w, locale.Text(r.Context(), "missing or invalid MCP authentication token"), http.StatusUnauthorized)
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

	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	expectedHash := sha256.Sum256([]byte(expected))
	gotHash := sha256.Sum256([]byte(parts[1]))
	return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
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
	if (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
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

func parseHTTPAddr(raw string) (scheme, listenAddr, basePath, displayAddr string, err error) {
	if raw == "" {
		return "", "", "", "", errors.New("addr must not be empty")
	}

	parsedInput := raw
	if !strings.Contains(parsedInput, "://") {
		parsedInput = "http://" + parsedInput
	}

	u, err := url.Parse(parsedInput)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid addr %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", "", "", fmt.Errorf("addr %q must use http or https", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", "", fmt.Errorf("addr %q must not contain credentials, query, or fragment", raw)
	}
	host := u.Host
	if host == "" {
		return "", "", "", "", fmt.Errorf("addr %q must include host and port (e.g. http://localhost:8000)", raw)
	}

	path := u.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	display := fmt.Sprintf("%s://%s%s", u.Scheme, host, path)
	return u.Scheme, host, path, display, nil
}

func main() {
	var (
		transport         string
		mode              string
		listenAddress     string
		panelSocket       string
		keyID             string
		showVersion       bool
		showCatalog       bool
		checkHealth       bool
		accessToken       string
		host              string
		addr              string
		mcpToken          string
		allowedOriginsRaw string
		allowInsecureHTTP bool
		allowRemoteHTTP   bool
		accessLevelRaw    string
		tlsDir            string
		tlsHostsRaw       string
	)
	flag.StringVar(&mode, "mode", "legacy", "Startup mode: legacy (independent v2 API Key) or panel (managed Core bridge)")
	flag.StringVar(&listenAddress, "listen", "127.0.0.1:18081", "Managed adapter loopback listen address")
	flag.StringVar(&panelSocket, "panel-socket", "/etc/1panel/mcp-runtime/core.sock", "Managed Core runtime Unix socket")
	flag.StringVar(&keyID, "key-id", "", "1Panel API Key ID (or PANEL_KEY_ID)")
	flag.BoolVar(&showVersion, "version", false, "Print adapter version and contract as JSON")
	flag.BoolVar(&showCatalog, "catalog", false, "Print the canonical v2 tool catalog as JSON")
	flag.BoolVar(&checkHealth, "healthcheck", false, "Check the managed adapter and Core bridge")
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio, streamable-http)")
	flag.StringVar(&addr, "addr", "http://127.0.0.1:8000", "Base URL (host, port, optional path) for HTTP transports")
	flag.StringVar(&accessToken, "token", "", "1Panel api key")
	flag.StringVar(&host, "host", "", "1Panel host (example:http://127.0.0.1:9999)")
	flag.StringVar(&mcpToken, "mcp-token", "", "Pre-shared Bearer token for streamable-http transport")
	flag.StringVar(&allowedOriginsRaw, "allowed-origins", "", "Comma-separated HTTP Origin allowlist for HTTP transports")
	flag.BoolVar(&allowInsecureHTTP, "allow-insecure-http", false, "Allow unauthenticated HTTP transports; only use for local development")
	flag.BoolVar(&allowRemoteHTTP, "allow-remote-http", false, "Allow HTTPS transports to listen on non-loopback addresses")
	flag.StringVar(&accessLevelRaw, "access-level", "", "MCP tool access level: readonly, readwrite, or full")
	flag.StringVar(&tlsDir, "tls-dir", "", "Directory for the MCP local CA and server certificate")
	flag.StringVar(&tlsHostsRaw, "tls-hosts", "", "Comma-separated DNS names and IP addresses for the MCP server certificate")
	flag.Parse()

	if mode != "legacy" && mode != "panel" {
		configurationError("mode must be legacy or panel")
	}
	if showVersion {
		printJSON(panel.Info(Version))
		return
	}
	if showCatalog {
		printJSON(panel.LocalCatalog())
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if checkHealth {
		if err := panel.CheckHealth(ctx, listenAddress); err != nil {
			fmt.Fprintf(os.Stderr, "managed health check failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if mode == "panel" {
		if host != "" || accessToken != "" || keyID != "" || mcpToken != "" || allowedOriginsRaw != "" || allowInsecureHTTP || allowRemoteHTTP || accessLevelRaw != "" || tlsDir != "" || tlsHostsRaw != "" || transport != "stdio" || addr != "http://127.0.0.1:8000" {
			configurationError("panel mode accepts only managed listen and socket settings")
		}
		err := panel.Run(ctx, panel.Config{ListenAddress: listenAddress, PanelSocket: panelSocket, Version: Version})
		if err != nil {
			fmt.Fprintf(os.Stderr, "managed adapter error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if host == "" {
		host = os.Getenv("PANEL_HOST")
	}
	if accessToken == "" {
		accessToken = os.Getenv("PANEL_ACCESS_TOKEN")
	}
	if keyID == "" {
		keyID = os.Getenv("PANEL_KEY_ID")
	}
	client, err := legacy.NewClient(legacy.Config{Host: host, APIKey: accessToken, KeyID: keyID})
	if err != nil {
		configurationError(err.Error())
	}
	defer client.Close()

	if mcpToken == "" {
		mcpToken = os.Getenv("MCP_AUTH_TOKEN")
	}
	if accessLevelRaw == "" {
		accessLevelRaw = os.Getenv("MCP_ACCESS_LEVEL")
	}
	accessLevel, err := parseAccessLevel(accessLevelRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	security := httpSecurityConfig{
		Token:          mcpToken,
		AllowedOrigins: parseAllowedOrigins(allowedOriginsRaw),
		AllowInsecure:  allowInsecureHTTP,
		AllowRemote:    allowRemoteHTTP,
		TLSDir:         tlsDir,
		TLSHosts:       parseTLSHosts(tlsHostsRaw),
	}

	if err := runServer(ctx, transport, addr, accessLevel, security, client); err != nil {
		fmt.Fprintf(os.Stderr, "server run error: %v\n", err)
		os.Exit(1)
	}
}

func printJSON(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, "unable to encode adapter metadata")
		os.Exit(1)
	}
}
func configurationError(message string) {
	fmt.Fprintf(os.Stderr, "configuration error: %s\n", message)
	os.Exit(2)
}
