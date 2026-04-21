/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mirth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Mirth Connect REST API.
type Client interface {
	GetServerStatus(ctx context.Context) (*ServerStatusResponse, error)
	GetSystemStats(ctx context.Context) (*SystemStats, error)
	GetChannelStatuses(ctx context.Context) ([]DashboardStatus, error)
	GetChannelStatistics(ctx context.Context, channelID string) (*ChannelStatistics, error)
	GetEvents(ctx context.Context, sinceID int64, limit int) ([]ServerEvent, error)
	RestartChannel(ctx context.Context, channelID string) error
	StartChannel(ctx context.Context, channelID string) error
}

// ClientConfig holds configuration for the Mirth REST API client.
type ClientConfig struct {
	BaseURL            string
	Username           string
	Password           string
	InsecureSkipVerify bool
}

type httpClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new Mirth REST API client.
func NewClient(cfg ClientConfig) Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // user-configurable for self-signed certs
		},
	}

	return &httpClient{
		baseURL:  cfg.BaseURL,
		username: cfg.Username,
		password: cfg.Password,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}
}

func (c *httpClient) doRequest(ctx context.Context, method, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest") // CSRF protection

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *httpClient) GetServerStatus(ctx context.Context) (*ServerStatusResponse, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/api/server/status")
	if err != nil {
		return nil, fmt.Errorf("getting server status: %w", err)
	}

	var status ServerStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("unmarshaling server status: %w", err)
	}

	return &status, nil
}

func (c *httpClient) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/api/system/stats")
	if err != nil {
		return nil, fmt.Errorf("getting system stats: %w", err)
	}

	var resp SystemStatsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling system stats: %w", err)
	}

	return &resp.Stats, nil
}

func (c *httpClient) GetChannelStatuses(ctx context.Context) ([]DashboardStatus, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/api/channels/statuses")
	if err != nil {
		return nil, fmt.Errorf("getting channel statuses: %w", err)
	}

	var resp DashboardStatusListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling channel statuses: %w", err)
	}

	if resp.List == nil {
		return nil, nil
	}

	return resp.List.DashboardStatuses, nil
}

func (c *httpClient) GetChannelStatistics(ctx context.Context, channelID string) (*ChannelStatistics, error) {
	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/channels/%s/statistics", channelID))
	if err != nil {
		return nil, fmt.Errorf("getting channel statistics: %w", err)
	}

	var stats ChannelStatistics
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("unmarshaling channel statistics: %w", err)
	}

	return &stats, nil
}

func (c *httpClient) GetEvents(ctx context.Context, sinceID int64, limit int) ([]ServerEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	path := fmt.Sprintf("/api/events?offset=0&limit=%d&minEventId=%d", limit, sinceID)
	body, err := c.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("getting events: %w", err)
	}

	events, err := ParseServerEvents(body)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling events: %w", err)
	}
	return events, nil
}

func (c *httpClient) RestartChannel(ctx context.Context, channelID string) error {
	_, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/channels/%s/_restart", channelID))
	if err != nil {
		return fmt.Errorf("restarting channel %s: %w", channelID, err)
	}
	return nil
}

func (c *httpClient) StartChannel(ctx context.Context, channelID string) error {
	_, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/channels/%s/_start", channelID))
	if err != nil {
		return fmt.Errorf("starting channel %s: %w", channelID, err)
	}
	return nil
}
