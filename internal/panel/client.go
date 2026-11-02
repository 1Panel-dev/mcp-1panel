// Package panel implements the dedicated, request-scoped 1Panel integration.
// It never uses the legacy process-wide API key or configurable API host.
package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

const (
	ContractVersion   = shared.ContractVersion
	ProtocolVersion   = "2025-03-26"
	AccessProofHeader = "X-Panel-MCP-Access"
	maxResponseSize   = 2 << 20
	runtimePrefix     = "/api/v2/core/mcp/runtime"
	catalogTimeout    = 30 * time.Second
	admissionTimeout  = 5 * time.Second
	completionTimeout = 2 * time.Second
	// Core owns a 330-second execution budget (Agent 310s, MySQL 300s).
	// Keep the adapter outside that budget so slow accepted writes can finish
	// their metadata and audit before the client observes an adapter timeout.
	invokeTimeout = 340 * time.Second
)

type ToolDefinition = shared.ToolDefinition

type Catalog struct {
	ContractVersion int              `json:"contractVersion"`
	Tools           []ToolDefinition `json:"tools"`
}

type Invocation struct {
	AdmissionID   string            `json:"admissionId,omitempty"`
	ToolID        string            `json:"toolId"`
	NodeID        string            `json:"nodeId"`
	SchemaVersion int               `json:"schemaVersion"`
	OperationID   string            `json:"operationId"`
	PathParams    map[string]string `json:"pathParams,omitempty"`
	PolicyHash    string            `json:"policyHash"`
	Arguments     json.RawMessage   `json:"arguments"`
	RequestID     string            `json:"requestId,omitempty"`
}

type Admission struct {
	ToolID        string          `json:"toolId"`
	NodeID        string          `json:"nodeId"`
	SchemaVersion int             `json:"schemaVersion"`
	PolicyHash    string          `json:"policyHash"`
	Arguments     json.RawMessage `json:"arguments"`
	RequestID     string          `json:"requestId,omitempty"`
}

type AdmissionResult struct {
	AdmissionID string    `json:"admissionId"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type InvocationResult struct {
	InvocationID string          `json:"invocationId"`
	NodeID       string          `json:"nodeId"`
	Result       json.RawMessage `json:"result"`
}

type RuntimeError = shared.ExecutionError

type Client struct {
	http *http.Client
}

type accessProofKey struct{}

// WithAccessProof carries Core's signed source proof only for this request.
// The adapter neither signs it nor derives identity from forwarding headers.
func WithAccessProof(ctx context.Context, proof string) context.Context {
	return context.WithValue(ctx, accessProofKey{}, proof)
}

// NewUnixClient connects only to the Core-owned Unix socket. Redirects cannot
// move the caller's credential to another endpoint, and environment proxies
// are deliberately not consulted.
func NewUnixClient(socketPath string) (*Client, error) {
	if !filepath.IsAbs(socketPath) || filepath.Clean(socketPath) != socketPath {
		return nil, errors.New("panel socket must be a clean absolute path")
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     30 * time.Second,
	}
	return &Client{http: &http.Client{
		Transport: transport,
		Timeout:   invokeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

// Health probes only the private bridge's readiness metadata. It neither
// authenticates as a user nor obtains an execution credential.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var health struct {
		Status          string `json:"status"`
		ContractVersion int    `json:"contractVersion"`
	}
	if err := c.request(ctx, "", http.MethodGet, "/healthz", nil, &health); err != nil {
		return err
	}
	if health.Status != "ok" || health.ContractVersion != ContractVersion {
		return contractError("Core MCP bridge is not ready or compatible")
	}
	return nil
}

func (c *Client) Tools(ctx context.Context, token string) (Catalog, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	var catalog Catalog
	if err := c.request(ctx, token, http.MethodGet, runtimePrefix+"/tools", nil, &catalog); err != nil {
		return catalog, err
	}
	if catalog.ContractVersion != ContractVersion {
		return catalog, contractError("Unsupported Core MCP contract version")
	}
	seen := make(map[string]bool, len(catalog.Tools))
	for i, tool := range catalog.Tools {
		local, ok := lookupTool(tool.ToolID)
		if !ok || seen[tool.ToolID] {
			return Catalog{}, contractError("Invalid Core MCP tool definition")
		}
		seen[tool.ToolID] = true
		definition := local.definition
		expectedHash := PolicyHash(definition)
		if tool.SchemaVersion != definition.SchemaVersion || PolicyHash(tool) != expectedHash || tool.PolicyHash != expectedHash {
			return Catalog{}, contractError("Core MCP tool policy does not match this adapter")
		}
		// Core returns an authorized subset. Tool schemas, descriptions and
		// handlers always come from this binary, never from remote metadata.
		definition.PolicyHash = expectedHash
		catalog.Tools[i] = definition
	}
	return catalog, nil
}

func (c *Client) Invoke(ctx context.Context, token string, invocation Invocation) (InvocationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()
	var result InvocationResult
	if err := c.request(ctx, token, http.MethodPost, runtimePrefix+"/invoke", invocation, &result); err != nil {
		return result, err
	}
	target := invocation.NodeID
	if target == "" {
		target = "local"
	}
	if result.InvocationID == "" || len(result.Result) == 0 || result.NodeID != target {
		return result, contractError("Invalid Core MCP invocation response")
	}
	return result, nil
}

// Admit reserves one whole tool call. Every later API operation is still
// authorized by Core; an admission is never shared with another tools/call.
func (c *Client) Admit(ctx context.Context, token string, admission Admission) (AdmissionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, admissionTimeout)
	defer cancel()
	var result AdmissionResult
	if err := c.request(ctx, token, http.MethodPost, runtimePrefix+"/admit", admission, &result); err != nil {
		// Decoding can fail after reading a valid ticket (for example, a bad
		// expiresAt). Do not leave that partial success reserved until expiry.
		if result.AdmissionID != "" && len(result.AdmissionID) <= 512 {
			_ = c.Complete(ctx, token, result.AdmissionID)
		}
		return AdmissionResult{}, err
	}
	if result.AdmissionID == "" || len(result.AdmissionID) > 512 || !result.ExpiresAt.After(time.Now()) {
		// A malformed success may still have reserved a slot. Release a received
		// ticket before failing closed instead of attempting any API operation.
		if result.AdmissionID != "" && len(result.AdmissionID) <= 512 {
			_ = c.Complete(ctx, token, result.AdmissionID)
		}
		return AdmissionResult{}, contractError("Invalid Core MCP admission response")
	}
	return result, nil
}

// Complete is cleanup, so a cancelled caller cannot prevent releasing the slot.
// Its independent short deadline also bounds cleanup when Core is unavailable.
func (c *Client) Complete(ctx context.Context, token, admissionID string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	return c.request(ctx, token, http.MethodPost, runtimePrefix+"/complete", struct {
		AdmissionID string `json:"admissionId"`
	}{admissionID}, nil)
}

// completeOutcome records adapter-local failures without a Core invocation.
// Successful calls and already-audited failures use Complete to release the ticket.
func (c *Client) completeOutcome(ctx context.Context, token, admissionID, errorCode string) (string, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	var result struct {
		InvocationID string `json:"invocationId"`
	}
	if err := c.request(ctx, token, http.MethodPost, runtimePrefix+"/complete", struct {
		AdmissionID string `json:"admissionId"`
		ErrorCode   string `json:"errorCode"`
	}{admissionID, errorCode}, &result); err != nil {
		return "", err
	}
	if result.InvocationID == "" || len(result.InvocationID) > 128 {
		return "", contractError("Invalid Core MCP response")
	}
	return result.InvocationID, nil
}

func (c *Client) request(ctx context.Context, token, method, path string, payload, result any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return &RuntimeError{Code: "MCP_INVALID_ARGUMENTS", Message: "Invalid tool arguments", HTTPStatus: http.StatusBadRequest}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if proof, _ := ctx.Value(accessProofKey{}).(string); proof != "" {
		req.Header.Set(AccessProofHeader, proof)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &RuntimeError{Code: "MCP_NODE_UNAVAILABLE", Message: "Cannot connect to the 1Panel MCP runtime", HTTPStatus: http.StatusServiceUnavailable}
	}
	defer resp.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil || len(encoded) > maxResponseSize {
		return contractError("Invalid Core MCP response")
	}
	if resp.StatusCode != http.StatusOK && !(result == nil && resp.StatusCode == http.StatusNoContent) {
		failure := RuntimeError{HTTPStatus: resp.StatusCode}
		if err := json.Unmarshal(encoded, &failure); err != nil || failure.Code == "" || failure.Message == "" {
			return &RuntimeError{Code: "MCP_EXECUTION_FAILED", Message: "The 1Panel MCP runtime rejected the request", HTTPStatus: http.StatusBadGateway}
		}
		return &failure
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(encoded, result); err != nil {
		return contractError("Invalid Core MCP response")
	}
	return nil
}

func contractError(message string) error {
	return &RuntimeError{Code: "MCP_SCHEMA_MISMATCH", Message: message, HTTPStatus: http.StatusBadGateway}
}

func isObject(value json.RawMessage) bool {
	value = bytes.TrimSpace(value)
	return len(value) > 0 && value[0] == '{' && json.Valid(value)
}

func runtimeFailure(err error) *RuntimeError {
	var failure *RuntimeError
	if errors.As(err, &failure) {
		return failure
	}
	return &RuntimeError{Code: "MCP_EXECUTION_FAILED", Message: "The 1Panel MCP request failed", HTTPStatus: http.StatusBadGateway}
}
