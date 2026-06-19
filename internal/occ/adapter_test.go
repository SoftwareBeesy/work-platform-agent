package occ

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubSyncInvoker struct {
	calls int
	err   error
	out   map[string]any
}

func (s *stubSyncInvoker) RunSync(ctx context.Context, args []string, stdin string) (map[string]any, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.out, nil
}

func TestExecRetriesOnRetryableError(t *testing.T) {
	t.Parallel()

	invoker := &stubSyncInvoker{err: errors.New("manage sync exec: exit code 2 (retryable)")}
	adapter := &Adapter{manage: invoker, timeout: time.Second, maxRetry: 3}

	_, err := adapter.Exec(context.Background(), "acme", "config:app:get", []string{"mework360_memail", "forceSSO"})
	if err == nil {
		t.Fatal("expected error")
	}
	if invoker.calls != 3 {
		t.Fatalf("calls = %d, want 3", invoker.calls)
	}
}

func TestExecReturnsParsedJSON(t *testing.T) {
	t.Parallel()

	invoker := &stubSyncInvoker{out: map[string]any{"value": "yes"}}
	adapter := &Adapter{manage: invoker, timeout: time.Second, maxRetry: 1}

	parsed, err := adapter.Exec(context.Background(), "acme", "config:app:get", []string{"mework360_memail", "forceSSO"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if parsed["value"] != "yes" {
		t.Fatalf("value = %v", parsed["value"])
	}
}

func TestExecRequiresTenantSlug(t *testing.T) {
	t.Parallel()

	adapter := &Adapter{manage: &stubSyncInvoker{}, timeout: time.Second, maxRetry: 1}
	_, err := adapter.Exec(context.Background(), " ", "user:list", nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
