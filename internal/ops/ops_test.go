package ops

import (
	"context"
	"testing"

	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
)

type stubManageAsync struct {
	args   []string
	stdin  string
	result map[string]any
	err    error
}

func (s *stubManageAsync) RunAsync(ctx context.Context, args []string, stdin string) (map[string]any, error) {
	s.args = append([]string(nil), args...)
	s.stdin = stdin
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubOccExec struct {
	calls [][3]string
	err   error
	out   map[string]any
}

func (s *stubOccExec) Exec(ctx context.Context, tenantSlug, subcmd string, args []string) (map[string]any, error) {
	s.calls = append(s.calls, [3]string{tenantSlug, subcmd, joinArgs(args)})
	if s.err != nil {
		return nil, s.err
	}
	if s.out != nil {
		return s.out, nil
	}
	return map[string]any{"value": ""}, nil
}

func joinArgs(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}

func TestHandleTenantCreateTypedPayload(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "job-typed-1"}}
	var events []contract.Event
	emit := func(ctx context.Context, event contract.Event) error {
		events = append(events, event)
		return nil
	}

	cmd := contract.Command{
		OperationID: "op-create",
		Payload: map[string]interface{}{
			"tenant_slug":      "acme",
			"domain":           "acme.example.com",
			"idempotency_key":  "idem-1",
			"callback_url":     "https://api.example/hook",
			"apps":             []interface{}{"mework360_memail", "me360_theme"},
		},
	}

	if err := HandleTenantCreate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleTenantCreate: %v", err)
	}

	if len(manageStub.args) < 3 || manageStub.args[2] != "create" {
		t.Fatalf("unexpected manage args: %v", manageStub.args)
	}
	if len(events) != 2 || events[1].Step != "dispatched" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestHandleMemailConfigureIdempotentSkip(t *testing.T) {
	t.Parallel()

	occStub := &stubOccExec{out: map[string]any{"value": "https://rc.example/roundcube"}}
	var events []contract.Event
	emit := func(ctx context.Context, event contract.Event) error {
		events = append(events, event)
		return nil
	}

	cmd := contract.Command{
		OperationID: "op-mail",
		Payload: map[string]interface{}{
			"tenant_slug":        "acme",
			"external_location":  "https://rc.example/roundcube",
			"force_sso":          true,
			"email_address_choice": "multiProfile",
			"disable_core_mail_app": true,
		},
	}

	if err := HandleMemailConfigure(context.Background(), cmd, "farm-1", occStub, emit); err != nil {
		t.Fatalf("HandleMemailConfigure: %v", err)
	}

	if len(events) == 0 || events[len(events)-1].Step != "memail_configured" {
		t.Fatalf("expected memail_configured event, got %+v", events)
	}
}
