package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/config"
	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/transport"
)

func TestPollCommandsAndPostEvent(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotFarmHeader string
	var posted contract.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotFarmHeader = r.Header.Get("X-Farm-Id")

		switch r.URL.Path {
		case "/api/agent/v1/commands":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", r.Method)
			}
			if r.URL.Query().Get("farm_id") != "farm-test" {
				t.Fatalf("unexpected farm_id query: %q", r.URL.Query().Get("farm_id"))
			}
			_ = json.NewEncoder(w).Encode(contract.CommandsResponse{
				SchemaVersion: 1,
				Commands: []contract.Command{{
					SchemaVersion: 1,
					OperationID:   "op-1",
					Operation:     "agent.ping",
					FarmID:        "farm-test",
				}},
			})
		case "/api/agent/v1/events":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cfg := config.Config{
		FarmID:          "farm-test",
		ControlPlaneURL: srv.URL,
		AgentToken:      "secret-token",
		PollTimeout:     5 * time.Second,
	}

	client := transport.NewWithHTTPClient(cfg, srv.Client())

	commands, err := client.PollCommands(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(commands) != 1 || commands[0].Operation != "agent.ping" {
		t.Fatalf("unexpected commands: %+v", commands)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotFarmHeader != "farm-test" {
		t.Fatalf("unexpected farm header: %q", gotFarmHeader)
	}

	event := contract.Event{
		SchemaVersion: 1,
		FarmID:        "farm-test",
		State:         "succeeded",
		Step:          "pong",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := client.PostEvent(context.Background(), event); err != nil {
		t.Fatalf("post event: %v", err)
	}
	if posted.Step != "pong" {
		t.Fatalf("unexpected posted event: %+v", posted)
	}
}
