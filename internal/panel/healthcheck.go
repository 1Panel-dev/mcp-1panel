package panel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// CheckHealth is a credential-free container probe. The adapter's /healthz
// includes the Core bridge, so a live process alone is not considered ready.
func CheckHealth(ctx context.Context, address string) error {
	if err := validateListen(address); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DisableCompression: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return checkHealth(ctx, client, address)
}

func checkHealth(ctx context.Context, client *http.Client, address string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/healthz", nil)
	if err != nil {
		return errors.New("MCP health check failed")
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("MCP health endpoint is unavailable")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	var info VersionInfo
	if err != nil || len(data) > 4096 || resp.StatusCode != http.StatusOK || json.Unmarshal(data, &info) != nil || info.Status != "ok" || info.AdapterVersion == "" || info.ContractVersion != ContractVersion || info.Protocol != "streamable-http" {
		return errors.New("MCP adapter or Core bridge is not ready or compatible")
	}
	return nil
}
