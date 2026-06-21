package ops

import (
	"context"
	"strings"
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

type eventCollector struct {
	events []contract.Event
}

func (c *eventCollector) emit(ctx context.Context, event contract.Event) error {
	c.events = append(c.events, event)
	return nil
}

func newEventCollector() (*eventCollector, EventEmitter) {
	c := &eventCollector{}
	return c, c.emit
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestHandleCustomAppsUpdateRingInvokesManage(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "job-ring-1"}}
	collector, emit := newEventCollector()

	cmd := contract.Command{
		OperationID: "op-ca-ring",
		Payload: map[string]interface{}{
			"ring": "canary",
			"json": true,
		},
	}

	if err := HandleCustomAppsUpdate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleCustomAppsUpdate: %v", err)
	}

	if !hasArg(manageStub.args, "custom-apps") || !hasArg(manageStub.args, "update") {
		t.Fatalf("expected custom-apps update argv, got %v", manageStub.args)
	}
	if !hasArg(manageStub.args, "--ring=canary") {
		t.Fatalf("expected --ring=canary, got %v", manageStub.args)
	}
	if !hasArg(manageStub.args, "--json") {
		t.Fatalf("expected --json when payload json is true, got %v", manageStub.args)
	}
	if len(collector.events) == 0 || collector.events[len(collector.events)-1].Step != "dispatched" {
		t.Fatalf("expected dispatched event, got %+v", collector.events)
	}
}

func TestHandleCustomAppsUpdateTenantInvokesManage(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "job-tenant-1"}}
	collector, emit := newEventCollector()

	cmd := contract.Command{
		OperationID: "op-ca-tenant",
		Payload: map[string]interface{}{
			"tenant": "acme-demo",
		},
	}

	if err := HandleCustomAppsUpdate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleCustomAppsUpdate: %v", err)
	}

	if !hasArg(manageStub.args, "--tenant=acme-demo") {
		t.Fatalf("expected --tenant=acme-demo, got %v", manageStub.args)
	}
	if hasArg(manageStub.args, "--ring=canary") || hasArg(manageStub.args, "--ring=stable") {
		t.Fatalf("tenant update must not include ring flag, got %v", manageStub.args)
	}
	if len(collector.events) == 0 || collector.events[len(collector.events)-1].Step != "dispatched" {
		t.Fatalf("expected dispatched event, got %+v", collector.events)
	}
}

func TestHandleCustomAppsUpdateWithAppID(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "job-app-1"}}
	_, emit := newEventCollector()

	cmd := contract.Command{
		OperationID: "op-ca-app",
		Payload: map[string]interface{}{
			"tenant":  "acme-demo",
			"app_id":  "mework360_memail",
			"json":    true,
		},
	}

	if err := HandleCustomAppsUpdate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleCustomAppsUpdate: %v", err)
	}

	if !hasArg(manageStub.args, "--app=mework360_memail") {
		t.Fatalf("expected --app=mework360_memail, got %v", manageStub.args)
	}
}

func TestHandleCustomAppsUpdateRequiresRingOrTenant(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "unused"}}
	collector, emit := newEventCollector()

	cmd := contract.Command{
		OperationID: "op-ca-missing-target",
		Payload:     map[string]interface{}{"json": true},
	}

	if err := HandleCustomAppsUpdate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleCustomAppsUpdate: %v", err)
	}

	if len(manageStub.args) != 0 {
		t.Fatalf("expected no manage invocation, got %v", manageStub.args)
	}
	if len(collector.events) == 0 || collector.events[len(collector.events)-1].Step != "validate" {
		t.Fatalf("expected validate failure, got %+v", collector.events)
	}
	if msg := collector.events[len(collector.events)-1].Message; !strings.Contains(strings.ToLower(msg), "ring") &&
		!strings.Contains(strings.ToLower(msg), "tenant") {
		t.Fatalf("expected ring/tenant validation message, got %q", msg)
	}
}

func TestHandleCustomAppsUpdateRejectsRingAndTenant(t *testing.T) {
	t.Parallel()

	manageStub := &stubManageAsync{result: map[string]any{"job_id": "unused"}}
	collector, emit := newEventCollector()

	cmd := contract.Command{
		OperationID: "op-ca-both",
		Payload: map[string]interface{}{
			"ring":   "canary",
			"tenant": "acme-demo",
		},
	}

	if err := HandleCustomAppsUpdate(context.Background(), cmd, "farm-1", manageStub, emit); err != nil {
		t.Fatalf("HandleCustomAppsUpdate: %v", err)
	}

	if len(manageStub.args) != 0 {
		t.Fatalf("expected no manage invocation, got %v", manageStub.args)
	}
	if len(collector.events) == 0 || collector.events[len(collector.events)-1].Step != "validate" {
		t.Fatalf("expected validate failure, got %+v", collector.events)
	}
}
