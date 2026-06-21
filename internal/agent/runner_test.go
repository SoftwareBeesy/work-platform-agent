package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/config"
	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/queue"
)

type stubManageInvoker struct {
	args   []string
	stdin  string
	result map[string]any
	err    error
}

func (s *stubManageInvoker) RunAsync(ctx context.Context, args []string, stdin string) (map[string]any, error) {
	s.args = append([]string(nil), args...)
	s.stdin = stdin
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type fakeTransport struct {
	mu       sync.Mutex
	commands []contract.Command
	events   []contract.Event
	postErr  error
}

func (f *fakeTransport) PollCommands(ctx context.Context, timeout time.Duration) ([]contract.Command, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.commands) == 0 {
		return nil, nil
	}
	out := f.commands
	f.commands = nil
	return out, nil
}

func (f *fakeTransport) PostEvent(ctx context.Context, event contract.Event) error {
	if f.postErr != nil {
		return f.postErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
	return nil
}

type memQueue struct {
	rows []queue.Row
	next int64
}

func (m *memQueue) Enqueue(ctx context.Context, payload string) error {
	m.next++
	m.rows = append(m.rows, queue.Row{ID: m.next, Payload: payload})
	return nil
}

func (m *memQueue) DequeueAll(ctx context.Context) ([]queue.Row, error) {
	return append([]queue.Row(nil), m.rows...), nil
}

func (m *memQueue) Delete(ctx context.Context, id int64) error {
	filtered := m.rows[:0]
	for _, row := range m.rows {
		if row.ID != id {
			filtered = append(filtered, row)
		}
	}
	m.rows = filtered
	return nil
}

func (m *memQueue) BumpAttempts(ctx context.Context, id int64) error {
	return nil
}

func TestRunnerHandlesPing(t *testing.T) {
	t.Parallel()

	tp := &fakeTransport{
		commands: []contract.Command{{
			OperationID: "op-ping",
			Operation:   "agent.ping",
		}},
	}
	q := &memQueue{}
	cfg := config.Config{
		FarmID:         "farm-test",
		PollTimeout:    time.Second,
		HeartbeatEvery: time.Hour,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}

	runner := NewRunner(cfg, tp, q, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = runner.Run(ctx) }()
	time.Sleep(400 * time.Millisecond)
	cancel()

	tp.mu.Lock()
	defer tp.mu.Unlock()
	if len(tp.events) == 0 {
		t.Fatal("expected at least one posted event")
	}
	foundPong := false
	for _, ev := range tp.events {
		if ev.Step == "pong" && ev.State == "succeeded" {
			foundPong = true
		}
	}
	if !foundPong {
		t.Fatalf("expected pong event, got %+v", tp.events)
	}
}

func TestRunnerQueuesEventsWhenPostFails(t *testing.T) {
	t.Parallel()

	tp := &fakeTransport{postErr: errors.New("control plane down")}
	q := &memQueue{}
	cfg := config.Config{
		FarmID:         "farm-test",
		PollTimeout:    time.Second,
		HeartbeatEvery: time.Hour,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}
	runner := NewRunner(cfg, tp, q, nil)

	err := runner.emitEvent(context.Background(), contract.Event{
		SchemaVersion: 1,
		FarmID:        "farm-test",
		State:         "running",
		Step:          "heartbeat",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("emitEvent should queue on failure: %v", err)
	}
	if len(q.rows) != 1 {
		t.Fatalf("expected 1 queued row, got %d", len(q.rows))
	}

	tp.postErr = nil
	if err := runner.flushQueue(context.Background()); err != nil {
		t.Fatalf("flush queue: %v", err)
	}
	if len(q.rows) != 0 {
		t.Fatalf("queue should be empty after flush, got %d", len(q.rows))
	}
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if len(tp.events) != 1 {
		t.Fatalf("expected 1 delivered event, got %d", len(tp.events))
	}
}

func TestRunnerRoutesCustomAppsUpdate(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageInvoker{result: map[string]any{"job_id": "job-runner-ca-1"}}
	tp := &fakeTransport{}
	q := &memQueue{}
	cfg := config.Config{
		FarmID:         "farm-test",
		PollTimeout:    time.Second,
		HeartbeatEvery: time.Hour,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}

	runner := NewRunner(cfg, tp, q, nil)
	runner.manage = manageStub

	cmd := contract.Command{
		OperationID: "op-runner-ca",
		Operation:   "custom-apps.update",
		Payload: map[string]interface{}{
			"ring": "canary",
			"json": true,
		},
	}

	if err := runner.handleCommand(context.Background(), cmd); err != nil {
		t.Fatalf("handleCommand: %v", err)
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	for _, ev := range tp.events {
		if ev.Step == "unsupported" {
			t.Fatalf("custom-apps.update must be routed to handler, got unsupported: %+v", ev)
		}
	}
	if len(manageStub.args) < 3 {
		t.Fatalf("expected manage invocation for custom-apps.update, args=%v", manageStub.args)
	}
}
