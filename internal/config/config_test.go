package config

import "testing"

func TestLoadRequiresCoreEnv(t *testing.T) {
	t.Parallel()

	t.Setenv("FARM_ID", "")
	t.Setenv("CONTROL_PLANE_URL", "")
	t.Setenv("AGENT_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing env")
	}
}

func TestLoadSuccess(t *testing.T) {
	t.Setenv("FARM_ID", "farm-a")
	t.Setenv("CONTROL_PLANE_URL", "https://cp.example.com")
	t.Setenv("AGENT_TOKEN", "token")
	t.Setenv("AGENT_QUEUE_PATH", t.TempDir()+"/q.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.FarmID != "farm-a" {
		t.Fatalf("unexpected farm id: %q", cfg.FarmID)
	}
	if cfg.QueuePath == "" {
		t.Fatal("queue path should be set")
	}
}

func TestValidateTLSRequiresPair(t *testing.T) {
	cfg := Config{TLSCertFile: "/tmp/cert.pem"}
	if err := cfg.ValidateTLS(); err == nil {
		t.Fatal("expected tls validation error")
	}
}
