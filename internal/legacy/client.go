// Package legacy exposes the canonical toolset through a configured API Key.
// It uses existing public APIs; managed MCP authorization belongs to panel mode.
package legacy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

const (
	apiPrefix               = "/api/v2"
	invokeTimeout           = 340 * time.Second
	maxResponseSize         = 2 << 20
	uncertainOutcomeMessage = shared.UnknownWriteMessage
	abnormalNodeMessage     = shared.NodeAbnormalMessage
)

// Config is copied during construction; changing it cannot change an existing
// client's identity. HTTPClient permits an explicit transport for local tests.
type Config struct {
	Host       string
	APIKey     string
	KeyID      string
	HTTPClient *http.Client
}

type Client struct {
	host  string
	key   string
	keyID string
	http  *http.Client
	now   func() time.Time
}

func NewClient(config Config) (*Client, error) {
	u, err := url.Parse(config.Host)
	if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(u.Path, "\\\r\n") {
		return nil, errors.New("API host must be an HTTP(S) URL without credentials, query or fragment")
	}
	if u.Path != "" && path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/") && u.Path != "/" {
		return nil, errors.New("API host must have a clean URL path")
	}
	if config.APIKey == "" || strings.ContainsAny(config.APIKey, "\r\n") || strings.ContainsAny(config.KeyID, "\r\n") {
		return nil, errors.New("API Key configuration is invalid")
	}
	h := http.Client{}
	if config.HTTPClient != nil {
		h = *config.HTTPClient
	}
	transport := h.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if standard, ok := transport.(*http.Transport); ok {
		cloned := standard.Clone()
		cloned.Proxy = nil
		transport = cloned
	}
	h.Transport = transport
	// API Key mode never adds ambient session cookies to its explicit identity.
	h.Jar = nil
	h.Timeout = invokeTimeout
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	u.Path = strings.TrimSuffix(u.Path, "/")
	return &Client{host: u.String(), key: config.APIKey, keyID: config.KeyID, http: &h, now: time.Now}, nil
}

func (c *Client) Close() {
	if c != nil && c.http != nil {
		c.http.CloseIdleConnections()
	}
}

type requestOptions struct {
	NodeName   string
	NodeStatus string
	Mutation   bool
	AllowEmpty bool
	NullList   bool
}

type node struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	IsLocal bool   `json:"isLocal"`
}

// Node discovery is optional for local calls. Public node APIs route by name,
// so resolve a selected ID from a fresh response, never from caller input.
func (c *Client) nodes(ctx context.Context) ([]node, error) {
	data, err := c.request(ctx, http.MethodPost, apiPrefix+"/core/nodes/list", map[string]any{"type": "all"}, requestOptions{NodeName: "local", NullList: true})
	if err != nil {
		return nil, err
	}
	var items []struct {
		ID     uint64 `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &items) != nil || items == nil || len(items) > 10000 {
		return nil, failure("MCP_SCHEMA_MISMATCH")
	}
	result := make([]node, 0, len(items))
	seenIDs, seenNames := map[uint64]bool{}, map[string]bool{}
	for _, item := range items {
		if item.ID == 0 || item.Name == "" || len(item.Name) > 256 || strings.ContainsAny(item.Name, "\r\n") || item.Status == "" || seenIDs[item.ID] || seenNames[item.Name] {
			return nil, failure("MCP_SCHEMA_MISMATCH")
		}
		seenIDs[item.ID], seenNames[item.Name] = true, true
		id := strconv.FormatUint(item.ID, 10)
		if item.Name == "local" {
			id = "local"
		}
		result = append(result, node{ID: id, Name: item.Name, Status: item.Status, IsLocal: item.Name == "local"})
	}
	return result, nil
}

func (c *Client) resolveNode(ctx context.Context, id string) (node, error) {
	if id == "local" {
		return node{ID: "local", Name: "local", IsLocal: true}, nil
	}
	items, err := c.nodes(ctx)
	if err != nil {
		return node{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return node{}, failure("MCP_FORBIDDEN")
}

func (c *Client) request(ctx context.Context, method, endpoint string, payload any, options requestOptions) (json.RawMessage, error) {
	if c == nil || c.http == nil || c.key == "" {
		return nil, failure("MCP_UNAUTHENTICATED")
	}
	ctx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, failure("MCP_INVALID_ARGUMENTS")
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.host+endpoint, body)
	if err != nil {
		return nil, failure("MCP_EXECUTION_FAILED")
	}
	timestamp := strconv.FormatInt(c.now().Unix(), 10)
	sum := md5.Sum([]byte("1panel" + c.key + timestamp))
	req.Header.Set("1Panel-Token", hex.EncodeToString(sum[:]))
	req.Header.Set("1Panel-Timestamp", timestamp)
	if c.keyID != "" {
		req.Header.Set("1Panel-Key-ID", c.keyID)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("CurrentNode", url.QueryEscape(options.NodeName))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, transportFailure(options, "MCP_NODE_UNAVAILABLE")
	}
	defer resp.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil || len(encoded) > maxResponseSize {
		return nil, transportFailure(options, "MCP_SCHEMA_MISMATCH")
	}
	var envelope struct {
		Code      json.RawMessage `json:"code"`
		Data      json.RawMessage `json:"data"`
		ErrorCode string          `json:"errorCode"`
		Message   string          `json:"message"`
	}
	if json.Unmarshal(encoded, &envelope) != nil {
		if resp.StatusCode == http.StatusOK {
			return nil, transportFailure(options, "MCP_SCHEMA_MISMATCH")
		}
		failed := responseFailure(resp.StatusCode, nil)
		if options.Mutation && (resp.StatusCode >= 500 || failed.Code == "MCP_EXECUTION_FAILED") {
			return nil, transportFailure(options, "MCP_EXECUTION_FAILED")
		}
		return nil, failureWithNodeHint(failed, resp.StatusCode, options)
	}
	var code int
	if resp.StatusCode != http.StatusOK || json.Unmarshal(envelope.Code, &code) != nil || code != http.StatusOK {
		failed := responseFailure(resp.StatusCode, envelope.Code)
		if knownErrorCode(envelope.ErrorCode) {
			failed = failure(envelope.ErrorCode)
		}
		// A gateway or server error after submission cannot prove a write did
		// not commit. Report uncertainty and never retry the operation here.
		if options.Mutation && ((resp.StatusCode >= 500 && resp.StatusCode <= 599) || (code >= 500 && code <= 599) || failed.Code == "MCP_EXECUTION_FAILED") {
			// A recognized business message is only a hint. It cannot prove
			// that a submitted write did not commit or make it retryable.
			if hint := shared.BusinessErrorHint(method, endpoint, envelope.Message); hint != "" {
				return nil, shared.UnknownWriteWithHint(hint)
			}
			return nil, transportFailure(options, "MCP_EXECUTION_FAILED")
		}
		status := resp.StatusCode
		if status == http.StatusOK {
			status = code
		}
		// A gateway status must not turn an explicit API denial into a
		// stopped-service or abnormal-node diagnosis. Keep its original code.
		var diagnosticCode int
		_ = json.Unmarshal(envelope.Code, &diagnosticCode)
		if !options.Mutation && diagnosticCode >= 400 && diagnosticCode < 500 {
			return nil, failed
		}
		failed = failureWithNodeHint(failed, status, options)
		// Read-side diagnostics must never reinterpret an authorization denial
		// or an uncertain mutation. Only a recognized complete API error may
		// supply a more specific, fixed reason for this query.
		if !options.Mutation && status == http.StatusInternalServerError && failed.Code == "MCP_EXECUTION_FAILED" {
			if hint := shared.ReadErrorHint(method, endpoint, envelope.Message); hint != "" {
				copy := *failed
				copy.Message = hint
				failed = &copy
			}
		}
		return nil, failed
	}
	if string(envelope.Data) == "null" && options.NullList && !options.Mutation {
		// Public collection endpoints encode an empty Go slice as null.
		return json.RawMessage(`[]`), nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		if options.Mutation && options.AllowEmpty {
			return json.RawMessage(`{}`), nil
		}
		return nil, transportFailure(options, "MCP_SCHEMA_MISMATCH")
	}
	// The existing website-directory API is a scalar response. Normalize only
	// this declared read into an object before the regular field projection.
	// Other scalar endpoints remain unsupported.
	if method == http.MethodGet && endpoint == apiPrefix+"/settings/website/dir" {
		var directory string
		if json.Unmarshal(envelope.Data, &directory) != nil || directory == "" || len(directory) > 4096 || !path.IsAbs(directory) || strings.ContainsAny(directory, "\x00\r\n") {
			return nil, failure("MCP_SCHEMA_MISMATCH")
		}
		return json.Marshal(struct {
			WebsiteDir string `json:"websiteDir"`
		}{directory})
	}
	// All supported public operations return objects or arrays. Reject an HTML
	// page encoded as a string instead of passing it to a business handler.
	if envelope.Data[0] != '{' && envelope.Data[0] != '[' {
		return nil, transportFailure(options, "MCP_SCHEMA_MISMATCH")
	}
	return envelope.Data, nil
}

// Node status is a panel snapshot, not proof of the cause of this failure.
// Add context only to an explicit read-side server error; never reject a
// successful request or replace authorization failures or uncertain writes.
func failureWithNodeHint(failed *shared.ExecutionError, status int, options requestOptions) *shared.ExecutionError {
	if options.Mutation || status != http.StatusInternalServerError || failed.Code != "MCP_EXECUTION_FAILED" {
		return failed
	}
	switch strings.ToLower(options.NodeStatus) {
	case "unhealthy", "lost", "failed":
		failed.Message = abnormalNodeMessage
	}
	return failed
}

func transportFailure(options requestOptions, code string) *shared.ExecutionError {
	if options.Mutation {
		failed := failure("MCP_EXECUTION_FAILED")
		failed.Message = uncertainOutcomeMessage
		return failed
	}
	return failure(code)
}

// Error text is deliberately local: Core/network messages may contain request
// credentials, backend addresses or raw business response data.
func failure(code string) *shared.ExecutionError {
	f := &shared.ExecutionError{Code: code, HTTPStatus: http.StatusBadGateway}
	switch code {
	case "MCP_UNAUTHENTICATED":
		f.Message = "API Key authentication failed"
		f.HTTPStatus = http.StatusUnauthorized
	case "MCP_FORBIDDEN":
		f.Message = "The API Key or owner cannot perform this operation"
		f.HTTPStatus = http.StatusForbidden
	case "MCP_INVALID_ARGUMENTS":
		f.Message = "Invalid tool arguments"
		f.HTTPStatus = http.StatusBadRequest
	case "MCP_CONFLICT":
		f.Message = "The requested operation is currently unavailable"
		f.HTTPStatus = http.StatusConflict
	case "MCP_POLICY_CHANGED":
		f.Message = "The operation policy changed"
		f.HTTPStatus = http.StatusConflict
	case "MCP_NODE_UNAVAILABLE":
		f.Message = "Cannot reach the selected 1Panel node. Check the node status and Agent service."
		f.HTTPStatus = http.StatusServiceUnavailable
	case "MCP_RATE_LIMITED":
		f.Message = "The API Key MCP request limit was reached"
		f.HTTPStatus = http.StatusTooManyRequests
	case "MCP_SCHEMA_MISMATCH":
		f.Message = "Invalid 1Panel API response"
	default:
		f.Code = "MCP_EXECUTION_FAILED"
		f.Message = "The request to the selected 1Panel node failed. Check the node status and panel logs."
	}
	return f
}

func knownErrorCode(code string) bool {
	switch code {
	case "MCP_UNAUTHENTICATED", "MCP_FORBIDDEN", "MCP_INVALID_ARGUMENTS", "MCP_CONFLICT", "MCP_POLICY_CHANGED", "MCP_NODE_UNAVAILABLE", "MCP_SCHEMA_MISMATCH", "MCP_RATE_LIMITED", "MCP_EXECUTION_FAILED":
		return true
	}
	return false
}

func responseFailure(status int, encodedCode json.RawMessage) *shared.ExecutionError {
	var code string
	if json.Unmarshal(encodedCode, &code) == nil && knownErrorCode(code) {
		return failure(code)
	}
	var envelopeCode int
	if status == http.StatusOK && json.Unmarshal(encodedCode, &envelopeCode) == nil {
		status = envelopeCode
	}
	switch status {
	case http.StatusUnauthorized:
		return failure("MCP_UNAUTHENTICATED")
	case http.StatusForbidden, http.StatusPreconditionFailed:
		return failure("MCP_FORBIDDEN")
	case http.StatusBadRequest:
		return failure("MCP_INVALID_ARGUMENTS")
	case http.StatusConflict:
		return failure("MCP_CONFLICT")
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return failure("MCP_NODE_UNAVAILABLE")
	case http.StatusTooManyRequests:
		return failure("MCP_RATE_LIMITED")
	default:
		return failure("MCP_EXECUTION_FAILED")
	}
}
