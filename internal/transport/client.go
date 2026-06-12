package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/config"
	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
)

// Client talks to the control plane over outbound HTTPS (+ optional mTLS).
type Client struct {
	baseURL    string
	farmID     string
	token      string
	httpClient *http.Client
}

// New builds an HTTP client from agent configuration.
func New(cfg config.Config) (*Client, error) {
	if err := cfg.ValidateTLS(); err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
		if cfg.TLSCAFile != "" {
			caPEM, err := os.ReadFile(cfg.TLSCAFile)
			if err != nil {
				return nil, fmt.Errorf("read AGENT_TLS_CA: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, fmt.Errorf("parse AGENT_TLS_CA PEM")
			}
			tlsCfg.RootCAs = pool
		}
		transport.TLSClientConfig = tlsCfg
	}

	httpClient := &http.Client{
		Timeout:   cfg.PollTimeout + 10*time.Second,
		Transport: transport,
	}
	return NewWithHTTPClient(cfg, httpClient), nil
}

// NewWithHTTPClient builds a client with a custom HTTP client (tests).
func NewWithHTTPClient(cfg config.Config, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    cfg.ControlPlaneURL,
		farmID:     cfg.FarmID,
		token:      cfg.AgentToken,
		httpClient: httpClient,
	}
}

// PollCommands performs long-poll GET /api/agent/v1/commands.
func (c *Client) PollCommands(ctx context.Context, timeout time.Duration) ([]contract.Command, error) {
	endpoint, err := url.Parse(c.baseURL + "/api/agent/v1/commands")
	if err != nil {
		return nil, fmt.Errorf("parse commands url: %w", err)
	}
	q := endpoint.Query()
	q.Set("farm_id", c.farmID)
	q.Set("timeout", strconv.Itoa(int(timeout.Seconds())))
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build poll request: %w", err)
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll commands: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("poll commands status %d: %s", resp.StatusCode, string(body))
	}

	var payload contract.CommandsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode commands response: %w", err)
	}
	return payload.Commands, nil
}

// PostEvent sends POST /api/agent/v1/events.
func (c *Client) PostEvent(ctx context.Context, event contract.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/agent/v1/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("post event status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) applyAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Farm-Id", c.farmID)
}
