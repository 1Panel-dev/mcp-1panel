package utils

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	panelAccessToken string
	apiBase          string
	configMu         sync.RWMutex
	nowFunc          = time.Now
	panelHTTPClient  = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

const maxPanelResponseBody = 4 << 20

func md5Sum(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func SetAccessToken(token string) {
	configMu.Lock()
	panelAccessToken = token
	configMu.Unlock()
}

func SetHost(host string) {
	configMu.Lock()
	if host == "" {
		apiBase = ""
	} else {
		apiBase = fmt.Sprintf("%s%s", host, ApiBase)
	}
	configMu.Unlock()
}

func GetAccessToken() string {
	configMu.RLock()
	token := panelAccessToken
	configMu.RUnlock()
	if token != "" {
		return token
	}
	if envToken := os.Getenv("PANEL_ACCESS_TOKEN"); envToken != "" {
		SetAccessToken(envToken)
		return envToken
	}
	return ""
}

func GetApiBase() string {
	configMu.RLock()
	base := apiBase
	configMu.RUnlock()
	if base != "" {
		return base
	}
	if host := os.Getenv("PANEL_HOST"); host != "" {
		SetHost(host)
		configMu.RLock()
		base = apiBase
		configMu.RUnlock()
		return base
	}
	return ""
}

type PanelClient struct {
	Url       string
	Method    string
	Payload   interface{}
	Headers   map[string]string
	Response  *http.Response
	parsedUrl *url.URL
	Query     map[string]string
}

type Option func(client *PanelClient)

type ErrMsg struct {
	Message string `json:"message"`
}

type PanelError struct {
	Code    int
	Message string
	Details string
}

func (e *PanelError) Error() string {
	return fmt.Sprintf("Panel API error: %s (code: %d)", e.Message, e.Code)
}

func NewPanelError(code int, message, details string) *PanelError {
	return &PanelError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

func NewAPIError(statusCode int, body []byte) error {
	var errMsg ErrMsg
	if err := json.Unmarshal(body, &errMsg); err != nil {
		return NewPanelError(statusCode, http.StatusText(statusCode), "Panel API returned a non-JSON error response")
	}

	return NewPanelError(statusCode, http.StatusText(statusCode), sanitizePanelDetails(errMsg.Message))
}

func NewAuthError() error {
	return NewPanelError(401, "Unauthorized", "Panel access token is missing or invalid")
}

func IsAuthError(err error) bool {
	var panelErr *PanelError
	if errors.As(err, &panelErr) {
		return panelErr.Code == 401
	}
	return false
}

func NewNetworkError(err error) error {
	return NewPanelError(0, "Network Error", err.Error())
}

func IsNetworkError(err error) bool {
	var panelErr *PanelError
	if errors.As(err, &panelErr) {
		return panelErr.Code == 0
	}
	return false
}

func NewInternalError(err error) error {
	return NewPanelError(500, "Internal Error", err.Error())
}

func IsAPIError(err error) bool {
	var panelError *PanelError
	ok := errors.As(err, &panelError)
	return ok
}

func NewPanelClient(method, urlPath string, opts ...Option) *PanelClient {
	urlString := GetApiBase() + urlPath
	parsedUrl, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}

	client := &PanelClient{
		Method:    method,
		Url:       parsedUrl.String(),
		parsedUrl: parsedUrl,
		Headers:   make(map[string]string),
	}

	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithQuery(query map[string]interface{}) Option {
	return func(client *PanelClient) {
		parsedQuery := make(map[string]string)
		if query != nil {
			queryParams := client.parsedUrl.Query()
			for k, v := range query {
				parsedValue := ""
				switch v := v.(type) {
				case string:
					parsedValue = v
				case int:
					parsedValue = strconv.Itoa(v)
				case bool:
					parsedValue = strconv.FormatBool(v)
				}
				if parsedValue != "" {
					queryParams.Set(k, parsedValue)
					parsedQuery[k] = parsedValue
				}
			}
			client.parsedUrl.RawQuery = queryParams.Encode()
		}
		client.Url = client.parsedUrl.String()
		client.Query = parsedQuery
	}
}

func WithPayload(payload interface{}) Option {
	return func(client *PanelClient) {
		client.Payload = payload
	}
}

func WithHeaders(headers map[string]string) Option {
	return func(client *PanelClient) {
		if client.Headers == nil {
			client.Headers = make(map[string]string)
		}
		for k, v := range headers {
			client.Headers[k] = v
		}
	}
}

func (p *PanelClient) SetHeaders(headers map[string]string) *PanelClient {
	if p.Headers == nil {
		p.Headers = make(map[string]string)
	}
	for k, v := range headers {
		p.Headers[k] = v
	}
	return p
}

func (p *PanelClient) Do(ctx context.Context) (*PanelClient, error) {
	p.Response = nil
	var reqBody io.Reader

	if p.Payload != nil {
		_payload, err := json.Marshal(p.Payload)
		if err != nil {
			return nil, NewInternalError(err)
		}
		reqBody = bytes.NewReader(_payload)
	}

	req, err := http.NewRequestWithContext(ctx, p.Method, p.Url, reqBody)
	if err != nil {
		return nil, NewInternalError(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "panel-client Go/"+runtime.GOOS+"/"+runtime.GOARCH+"/"+runtime.Version())

	token := GetAccessToken()
	if token == "" {
		return nil, NewAuthError()
	}

	timestamp := strconv.FormatInt(nowFunc().Unix(), 10)
	req.Header.Set("1Panel-Token", md5Sum("1panel"+token+timestamp))
	req.Header.Set("1Panel-Timestamp", timestamp)

	for key, value := range p.Headers {
		req.Header.Set(key, value)
	}

	resp, err := panelHTTPClient.Do(req)
	if err != nil {
		return p, NewNetworkError(err)
	}

	p.Response = resp

	if !p.IsSuccess() {
		defer resp.Body.Close()
		body, readErr := readPanelResponseBody(resp.Body)
		if readErr != nil {
			return p, NewPanelError(resp.StatusCode, http.StatusText(resp.StatusCode), sanitizePanelDetails(readErr.Error()))
		}
		return p, NewAPIError(resp.StatusCode, body)
	}

	return p, nil
}

func sanitizePanelDetails(details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return "No error details available"
	}
	const maxErrorDetails = 512
	if len(details) > maxErrorDetails {
		return details[:maxErrorDetails] + "..."
	}
	return details
}

func resetPanelConfigForTest() {
	configMu.Lock()
	panelAccessToken = ""
	apiBase = ""
	nowFunc = time.Now
	configMu.Unlock()
}

func (p *PanelClient) IsSuccess() bool {
	if p.Response == nil {
		return false
	}
	return p.Response.StatusCode >= http.StatusOK && p.Response.StatusCode < http.StatusMultipleChoices
}

func (p *PanelClient) IsFail() bool {
	return !p.IsSuccess()
}

func (p *PanelClient) GetRespBody() ([]byte, error) {
	if p.Response == nil || p.Response.Body == nil {
		return nil, errors.New("response or response body is nil")
	}
	defer p.Response.Body.Close()
	return readPanelResponseBody(p.Response.Body)
}

func readPanelResponseBody(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, maxPanelResponseBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxPanelResponseBody {
		return nil, fmt.Errorf("Panel API response exceeds %d bytes", maxPanelResponseBody)
	}
	return data, nil
}

func (p *PanelClient) ParseJSON(v interface{}) error {
	body, err := p.GetRespBody()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (p *PanelClient) Request(ctx context.Context, object any) (*mcp.CallToolResult, error) {
	_, err := p.Do(ctx)
	if err != nil {
		return panelToolError(err)
	}

	body, err := p.GetRespBody()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to read response body: %s", err.Error())},
			},
			IsError: true,
		}, NewInternalError(err)
	}

	if len(bytes.TrimSpace(body)) > 0 {
		var envelope struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &envelope); err == nil && envelope.Code != 0 && envelope.Code != http.StatusOK {
			message := strings.TrimSpace(envelope.Message)
			if message == "" {
				message = "Panel API request failed"
			}
			return panelToolError(NewPanelError(envelope.Code, message, sanitizePanelDetails(message)))
		}
	}

	if object == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Operation completed successfully"},
			},
		}, nil
	}

	if err = json.Unmarshal(body, object); err != nil {
		errorMessage := fmt.Sprintf("Failed to parse response: %v", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
			IsError: true,
		}, NewInternalError(errors.New(errorMessage))
	}

	result, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to format response: %s", err.Error())},
			},
			IsError: true,
		}, NewInternalError(err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
		StructuredContent: object,
	}, nil
}

func panelToolError(err error) (*mcp.CallToolResult, error) {
	text := err.Error()
	switch {
	case IsAuthError(err):
		text = "Authentication failed: Please check your Panel access token"
	case IsNetworkError(err):
		text = "Network error: Unable to connect to Panel API"
	case IsAPIError(err):
		var panelErr *PanelError
		errors.As(err, &panelErr)
		text = fmt.Sprintf("API error (%d): %s", panelErr.Code, panelErr.Details)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}, err
}
