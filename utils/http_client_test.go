package utils

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestPanelRequestRejectsBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":500,"message":"database creation failed"}`)
	}))
	defer server.Close()
	configurePanelTest(t, server.URL)

	var response struct {
		Code int `json:"code"`
	}
	result, err := NewPanelClient(http.MethodPost, "/test").Request(context.Background(), &response)
	if err == nil || result == nil || !result.IsError {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	var panelErr *PanelError
	if !errors.As(err, &panelErr) || panelErr.Code != 500 {
		t.Fatalf("error = %#v, want PanelError code 500", err)
	}
}

func TestPanelRequestReturnsStructuredSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200,"data":{"value":"ready"}}`)
	}))
	defer server.Close()
	configurePanelTest(t, server.URL)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	result, err := NewPanelClient(http.MethodGet, "/test").Request(context.Background(), &response)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || result.StructuredContent != &response || response.Code != http.StatusOK || response.Data.Value != "ready" {
		t.Fatalf("result = %#v, response = %#v", result, response)
	}
}

func TestPanelRequestPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	configurePanelTest(t, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := NewPanelClient(http.MethodGet, "/test").Request(ctx, &struct{}{})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Panel request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || !IsNetworkError(err) {
			t.Fatalf("error = %v, want network cancellation error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Panel request ignored context cancellation")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream handler did not observe cancellation")
	}
}

func TestPanelHTTPErrorClosesBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(`{"message":"failed"}`)}
	originalClient := panelHTTPClient
	panelHTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       body,
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() { panelHTTPClient = originalClient })
	configurePanelTest(t, "http://panel.test")

	_, err := NewPanelClient(http.MethodGet, "/test").Do(context.Background())
	if err == nil {
		t.Fatal("HTTP error was accepted")
	}
	if !body.closed {
		t.Fatal("HTTP error response body was not closed")
	}
}

func TestPanelClientDoesNotFollowRedirect(t *testing.T) {
	redirectFollowed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == ApiBase+"/target" {
			redirectFollowed <- struct{}{}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, request, ApiBase+"/target", http.StatusFound)
	}))
	defer server.Close()
	configurePanelTest(t, server.URL)

	_, err := NewPanelClient(http.MethodGet, "/test").Do(context.Background())
	var panelErr *PanelError
	if !errors.As(err, &panelErr) || panelErr.Code != http.StatusFound {
		t.Fatalf("error = %#v, want PanelError code 302", err)
	}
	select {
	case <-redirectFollowed:
		t.Fatal("Panel client followed redirect")
	default:
	}
}

func TestPanelResponseBodyLimit(t *testing.T) {
	_, err := readPanelResponseBody(strings.NewReader(strings.Repeat("x", maxPanelResponseBody+1)))
	if err == nil {
		t.Fatal("oversized Panel response was accepted")
	}
}

func configurePanelTest(t *testing.T, host string) {
	t.Helper()
	resetPanelConfigForTest()
	SetHost(host)
	SetAccessToken("test-token")
	t.Cleanup(resetPanelConfigForTest)
}
