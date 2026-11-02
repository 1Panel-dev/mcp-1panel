package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/locale"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset"
	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxRequestSize = 1 << 20
	// Every batch shares one deadline, rather than multiplying the execution
	// allowance by the number of tools. The HTTP write budget includes the
	// separately bounded request-body read plus a response-writing margin.
	invokeBatchTimeout   = invokeTimeout + 5*time.Second
	invokeCatalogTimeout = 5 * time.Second
	requestReadTimeout   = 30 * time.Second
	responseWriteTimeout = requestReadTimeout + invokeBatchTimeout + 5*time.Second
)

type Config struct {
	ListenAddress string
	PanelSocket   string
	Version       string
}

type VersionInfo struct {
	Status          string `json:"status"`
	AdapterVersion  string `json:"adapterVersion"`
	ContractVersion int    `json:"contractVersion"`
	Protocol        string `json:"protocol"`
}

func Info(version string) VersionInfo {
	return VersionInfo{Status: "ok", AdapterVersion: version, ContractVersion: ContractVersion, Protocol: "streamable-http"}
}

// Run serves the panel integration on loopback; the public Core entrance owns
// TLS and reverse proxying. No API key or caller credential is accepted here as
// a startup setting.
func Run(ctx context.Context, cfg Config) error {
	if err := validateListen(cfg.ListenAddress); err != nil {
		return err
	}
	client, err := NewUnixClient(cfg.PanelSocket)
	if err != nil {
		return err
	}
	defer client.Close()
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           newHandler(client, cfg.Version),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       requestReadTimeout,
		WriteTimeout:      responseWriteTimeout,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(listener) }()
	log.Printf("1Panel MCP adapter listening on %s", cfg.ListenAddress)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownContext); err != nil {
			_ = srv.Close()
			return err
		}
		return nil
	}
}

func validateListen(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("panel listen address must be an explicit loopback IP and port")
	}
	ip := net.ParseIP(host)
	number, err := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || err != nil || number < 1 || number > 65535 {
		return errors.New("panel listen address must be an explicit loopback IP and valid port")
	}
	return nil
}

type handler struct {
	client   *Client
	version  string
	language locale.Language
}

func newHandler(client *Client, version string) http.Handler {
	return &handler{client: client, version: version, language: locale.Environment()}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := locale.RequestContext(r.Context(), r.Header.Get("Accept-Language"), nil, h.language)
	r = r.WithContext(ctx)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The first deployment supports non-browser MCP clients. With no configured
	// public origin, every browser origin must be rejected, including "null".
	if len(r.Header.Values("Origin")) > 0 {
		writeJSON(w, http.StatusForbidden, rpcError(ctx, nil, -32000, "Browser origins are not supported", nil))
		return
	}
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		info := Info(h.version)
		status := http.StatusOK
		if err := h.client.Health(r.Context()); err != nil {
			info.Status = "error"
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, info)
		return
	}
	if r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, rpcError(ctx, nil, -32000, "MCP authentication required", &RuntimeError{Code: "MCP_UNAUTHENTICATED", Message: "A dedicated MCP Bearer token is required"}))
		return
	}
	r = r.WithContext(WithAccessProof(r.Context(), r.Header.Get(AccessProofHeader)))
	if version := r.Header.Get("MCP-Protocol-Version"); version != "" && version != ProtocolVersion {
		writeJSON(w, http.StatusBadRequest, rpcError(ctx, nil, -32600, "Unsupported MCP protocol version", nil))
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, rpcError(ctx, nil, -32600, "Content-Type must be application/json", nil))
		return
	}
	if !accepts(r.Header.Get("Accept"), "application/json") || !accepts(r.Header.Get("Accept"), "text/event-stream") {
		writeJSON(w, http.StatusNotAcceptable, rpcError(ctx, nil, -32600, "Accept must include application/json and text/event-stream", nil))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestSize))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, rpcError(ctx, nil, -32600, "MCP request is too large", nil))
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 || !json.Valid(body) {
		writeJSON(w, http.StatusBadRequest, rpcError(ctx, nil, -32700, "Invalid JSON", nil))
		return
	}
	batch := body[0] == '['
	messages := []json.RawMessage{body}
	if batch {
		if err := json.Unmarshal(body, &messages); err != nil || len(messages) == 0 || len(messages) > 32 {
			writeJSON(w, http.StatusBadRequest, rpcError(ctx, nil, -32600, "A batch must contain between 1 and 32 messages", nil))
			return
		}
	}
	responses := make([]any, 0, len(messages))
	status := http.StatusOK
	ctx, cancel := context.WithTimeout(r.Context(), messagesTimeout(messages))
	defer cancel()
	for _, message := range messages {
		response, messageStatus := h.message(ctx, token, message)
		if response != nil {
			responses = append(responses, response)
		}
		if messageStatus != http.StatusOK {
			status = messageStatus
		}
	}
	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	if batch {
		writeJSON(w, status, responses)
	} else {
		writeJSON(w, status, responses[0])
	}
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func messagesTimeout(messages []json.RawMessage) time.Duration {
	for _, raw := range messages {
		var request message
		if json.Unmarshal(raw, &request) == nil && request.Method == "tools/call" {
			return invokeBatchTimeout
		}
	}
	return catalogTimeout
}

func (h *handler) message(ctx context.Context, token string, raw json.RawMessage) (any, int) {
	var request message
	if !isObject(raw) || json.Unmarshal(raw, &request) != nil || request.JSONRPC != "2.0" || !validID(request.ID) {
		return rpcError(ctx, nil, -32600, "Invalid JSON-RPC request", nil), http.StatusBadRequest
	}
	var languageParams struct {
		Meta map[string]any `json:"_meta"`
	}
	_ = json.Unmarshal(request.Params, &languageParams)
	ctx = locale.WithMetadata(ctx, languageParams.Meta)
	// Local handlers own tool semantics; Core authorizes every API operation,
	// including a known tool invoked without a preceding tools/list.
	if request.Method == "tools/call" && len(request.ID) > 0 {
		return h.invoke(ctx, token, request)
	}
	catalog, err := h.client.Tools(ctx, token)
	if err != nil {
		failure := runtimeFailure(err)
		return rpcError(ctx, request.ID, -32000, failure.Message, failure), failure.HTTPStatus
	}
	if len(request.ID) == 0 {
		if strings.HasPrefix(request.Method, "notifications/") {
			return nil, http.StatusOK
		}
		return rpcError(ctx, nil, -32600, "A request ID is required", nil), http.StatusBadRequest
	}
	if request.Method == "" && (len(request.Result) > 0 || len(request.Error) > 0) {
		return nil, http.StatusOK
	}
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string          `json:"protocolVersion"`
			Capabilities    json.RawMessage `json:"capabilities"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		if !isObject(request.Params) || json.Unmarshal(request.Params, &params) != nil || params.ProtocolVersion == "" || !isObject(params.Capabilities) || params.ClientInfo.Name == "" || params.ClientInfo.Version == "" {
			return rpcError(ctx, request.ID, -32602, "Invalid initialization parameters", nil), http.StatusOK
		}
		return rpcResult(request.ID, map[string]any{
			"protocolVersion": ProtocolVersion,
			"serverInfo":      map[string]string{"name": "1Panel MCP", "version": h.version},
			"capabilities":    map[string]any{"tools": map[string]bool{"listChanged": false}},
		}), http.StatusOK
	case "ping":
		return rpcResult(request.ID, struct{}{}), http.StatusOK
	case "tools/list":
		var params struct {
			Cursor string `json:"cursor"`
		}
		if len(request.Params) > 0 && (!isObject(request.Params) || json.Unmarshal(request.Params, &params) != nil || params.Cursor != "") {
			return rpcError(ctx, request.ID, -32602, "Tool pagination is not supported", nil), http.StatusOK
		}
		tools := make([]*mcp.Tool, 0, len(catalog.Tools))
		for _, tool := range catalog.Tools {
			tools = append(tools, &mcp.Tool{Name: tool.ToolID, Description: tool.Description, InputSchema: tool.InputSchema, OutputSchema: shared.ToolOutputSchema(tool), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: tool.ReadOnly}})
		}
		return rpcResult(request.ID, map[string]any{"tools": tools}), http.StatusOK
	default:
		return rpcError(ctx, request.ID, -32601, "Method not found", nil), http.StatusOK
	}
}

func (h *handler) invoke(ctx context.Context, token string, request message) (any, int) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if !isObject(request.Params) || json.Unmarshal(request.Params, &params) != nil || params.Name == "" {
		return rpcError(ctx, request.ID, -32602, "Invalid tool parameters", nil), http.StatusOK
	}
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage(`{}`)
	}
	if !isObject(params.Arguments) {
		return rpcError(ctx, request.ID, -32602, "Tool arguments must be an object", nil), http.StatusOK
	}
	requestID := string(request.ID)
	if len(request.ID) > 0 && request.ID[0] == '"' {
		_ = json.Unmarshal(request.ID, &requestID)
	}
	if len(requestID) > 128 {
		return rpcError(ctx, request.ID, -32602, "Request ID is too long", nil), http.StatusOK
	}
	tool, ok := lookupTool(params.Name)
	if !ok {
		return rpcError(ctx, request.ID, -32602, "Unknown managed tool", &RuntimeError{Code: "MCP_UNKNOWN_TOOL", Message: "Unknown managed tool"}), http.StatusOK
	}
	// Query grants per request without spending the write execution budget on
	// discovery. The outer request/batch deadline still bounds the whole call.
	catalogContext, cancel := context.WithTimeout(ctx, invokeCatalogTimeout)
	catalog, err := h.client.Tools(catalogContext, token)
	cancel()
	if err == nil {
		policyHash := PolicyHash(tool.definition)
		grantedTool := false
		for _, granted := range catalog.Tools {
			if granted.ToolID == params.Name {
				policyHash = granted.PolicyHash
				grantedTool = true
				break
			}
		}
		// Even when this known tool is absent from the authorized directory,
		// ask Core for admission with the local policy so its denial is audited. Directory
		// membership is never accepted as execution authorization.
		var result InvocationResult
		if !grantedTool {
			// A known but ungranted tool still reaches Core admission for auditing.
			// It must never bypass whole-call limits via a ticketless API operation.
			spec, _ := toolset.Lookup(params.Name)
			var admission AdmissionResult
			admission, _, err = admitManaged(ctx, h.client, token, spec, params.Arguments, requestID, policyHash)
			if err == nil {
				_ = h.client.Complete(ctx, token, admission.AdmissionID)
				err = contractError("Core accepted an unlisted tool")
			}
		} else {
			result, err = tool.invoke(ctx, h.client, token, params.Arguments, requestID, policyHash)
		}
		if err == nil {
			encoded, _ := json.Marshal(result)
			success := toolTextResult(string(encoded))
			success.StructuredContent = result
			return rpcResult(request.ID, success), http.StatusOK
		}
	}
	failure := runtimeFailure(err)
	failure = explainInvalidArguments(failure, tool.definition, params.Arguments)
	if failure.HTTPStatus == http.StatusUnauthorized || failure.Code == "MCP_DISABLED" {
		return rpcError(ctx, request.ID, -32000, failure.Message, failure), failure.HTTPStatus
	}
	failure = locale.Error(ctx, failure)
	encoded, _ := json.Marshal(failure)
	result := toolTextResult(string(encoded))
	result.IsError = true
	if failure.RetryAfterSeconds > 0 {
		result.Meta = mcp.Meta{"retryAfterSeconds": failure.RetryAfterSeconds}
	}
	return rpcResult(request.ID, result), http.StatusOK
}

// Core makes the authorization/admission decision first. Enrich only its
// existing invalid-arguments result, without changing the error classification,
// audit metadata, or allowing a diagnostic to bypass a denied operation.
func explainInvalidArguments(failure *RuntimeError, definition ToolDefinition, arguments json.RawMessage) *RuntimeError {
	if failure == nil || failure.Code != "MCP_INVALID_ARGUMENTS" {
		return failure
	}
	var schema shared.Schema
	var value any
	if json.Unmarshal(definition.InputSchema, &schema) != nil || json.Unmarshal(arguments, &value) != nil {
		return failure
	}
	err := shared.ValidateValue(schema, value)
	var detail *shared.ExecutionError
	if !errors.As(err, &detail) {
		return failure
	}
	if _, _, ok := shared.ValidationDetails(detail); !ok {
		return failure
	}
	localized := *detail
	localized.Code = failure.Code
	localized.HTTPStatus = failure.HTTPStatus
	localized.InvocationID = failure.InvocationID
	localized.RetryAfterSeconds = failure.RetryAfterSeconds
	return &localized
}

func bearerToken(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !strings.HasPrefix(parts[1], "mcp_") || len(parts[1]) > 512 || strings.Contains(parts[1], ",") {
		return "", false
	}
	return parts[1], true
}

func accepts(header, want string) bool {
	for _, value := range strings.Split(header, ",") {
		kind, params, err := mime.ParseMediaType(strings.TrimSpace(value))
		if err == nil && kind == want {
			if quality, ok := params["q"]; ok {
				q, err := strconv.ParseFloat(quality, 64)
				if err != nil || q <= 0 || q > 1 {
					continue
				}
			}
			return true
		}
	}
	return false
}

func validID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	if id[0] == '"' {
		var value string
		return json.Unmarshal(id, &value) == nil
	}
	var value json.Number
	return json.Unmarshal(id, &value) == nil && string(id) != "null"
}

func rpcResult(id json.RawMessage, result any) any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func rpcError(ctx context.Context, id json.RawMessage, code int, message string, data any) any {
	if failure, ok := data.(*RuntimeError); ok {
		localized := locale.Error(ctx, failure)
		data = localized
		if message == failure.Message {
			message = localized.Message
		}
	}
	detail := map[string]any{"code": code, "message": locale.Text(ctx, message)}
	if data != nil {
		detail["data"] = data
	}
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": detail}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		// Do not log payloads: arguments and runtime results can contain secrets.
		log.Print("MCP response could not be written")
	}
}

func toolTextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
