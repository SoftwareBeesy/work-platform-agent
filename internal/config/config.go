package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings for the farm agent daemon.
type Config struct {
	FarmID           string
	ControlPlaneURL  string
	AgentToken       string
	TLSCertFile      string
	TLSKeyFile       string
	TLSCAFile        string
	PollTimeout      time.Duration
	QueuePath        string
	HeartbeatEvery   time.Duration
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	ManageBinPath    string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		FarmID:          strings.TrimSpace(os.Getenv("FARM_ID")),
		ControlPlaneURL: strings.TrimRight(strings.TrimSpace(os.Getenv("CONTROL_PLANE_URL")), "/"),
		AgentToken:      strings.TrimSpace(os.Getenv("AGENT_TOKEN")),
		TLSCertFile:     strings.TrimSpace(os.Getenv("AGENT_TLS_CERT")),
		TLSKeyFile:      strings.TrimSpace(os.Getenv("AGENT_TLS_KEY")),
		TLSCAFile:       strings.TrimSpace(os.Getenv("AGENT_TLS_CA")),
		PollTimeout:     durationEnv("AGENT_POLL_TIMEOUT_SEC", 55*time.Second),
		QueuePath:       strings.TrimSpace(os.Getenv("AGENT_QUEUE_PATH")),
		HeartbeatEvery:  durationEnv("AGENT_HEARTBEAT_SEC", 30*time.Second),
		InitialBackoff:  durationEnv("AGENT_BACKOFF_INITIAL_SEC", 2*time.Second),
		MaxBackoff:      durationEnv("AGENT_BACKOFF_MAX_SEC", 2*time.Minute),
		ManageBinPath:   strings.TrimSpace(os.Getenv("NEXTCLOUD_MANAGE_PATH")),
	}

	if cfg.ManageBinPath == "" {
		cfg.ManageBinPath = "/usr/local/bin/nextcloud-manage"
	}

	if cfg.FarmID == "" {
		return cfg, errors.New("FARM_ID is required")
	}
	if cfg.ControlPlaneURL == "" {
		return cfg, errors.New("CONTROL_PLANE_URL is required")
	}
	if cfg.AgentToken == "" {
		return cfg, errors.New("AGENT_TOKEN is required")
	}
	if cfg.QueuePath == "" {
		cfg.QueuePath = "/var/lib/mework360-platform-agent/events.db"
	}

	return cfg, nil
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

// ValidateTLS returns an error when only one of cert/key is set.
func (c Config) ValidateTLS() error {
	hasCert := c.TLSCertFile != ""
	hasKey := c.TLSKeyFile != ""
	if hasCert != hasKey {
		return fmt.Errorf("AGENT_TLS_CERT and AGENT_TLS_KEY must both be set for mTLS")
	}
	return nil
}
