package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/manage"
)

// ManageAsync runs nextcloud-manage with --async --json.
type ManageAsync interface {
	RunAsync(ctx context.Context, args []string, stdin string) (map[string]any, error)
}

// EventEmitter posts typed operation progress to the control plane.
type EventEmitter func(ctx context.Context, event contract.Event) error

// HandleTenantCreate dispatches manage create --async from typed or legacy payload.
func HandleTenantCreate(
	ctx context.Context,
	cmd contract.Command,
	farmID string,
	manageInvoker ManageAsync,
	emit EventEmitter,
) error {
	args, stdin, err := tenantCreateArgs(cmd.Payload)
	if err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "validate", err.Error())
	}

	if err := emit(ctx, progressEvent(cmd, farmID, "running", "accepted", "tenant create accepted", 10)); err != nil {
		return err
	}

	parsed, err := manageInvoker.RunAsync(ctx, args, stdin)
	if err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "manage_exec", err.Error())
	}

	jobID, err := manage.JobID(parsed)
	if err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "parse_job_id", err.Error())
	}

	return emit(ctx, contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        farmID,
		State:         "succeeded",
		Step:          "dispatched",
		Message:       "tenant create job queued on farm",
		Percent:       100,
		Timestamp:     nowRFC3339(),
		EventType:     "progress",
		Data: map[string]any{
			"job_id":           jobID,
			"steps_completed":  []string{"accepted", "dispatched"},
			"steps_pending":    []string{"db_created", "containers_up", "apps_installed", "memail_configured", "readiness_probe", "ready"},
		},
	})
}

func tenantCreateArgs(payload map[string]interface{}) ([]string, string, error) {
	if payload == nil {
		return nil, "", fmt.Errorf("payload is required")
	}

	if rawArgs, ok := payload["args"].([]interface{}); ok && len(rawArgs) > 0 {
		return legacyArgs(rawArgs, payload)
	}

	return typedCreateArgs(payload)
}

func typedCreateArgs(payload map[string]interface{}) ([]string, string, error) {
	slug := stringField(payload, "tenant_slug")
	domain := stringField(payload, "domain")
	if slug == "" || domain == "" {
		return nil, "", fmt.Errorf("tenant_slug and domain are required")
	}

	args := []string{slug, domain, "create"}

	if key := stringField(payload, "idempotency_key"); key != "" {
		args = append(args, "--idempotency-key="+key)
	}
	if callback := stringField(payload, "callback_url"); callback != "" {
		args = append(args, "--callback="+callback)
	}
	if boolField(payload, "full_apps") {
		args = append(args, "--full-apps")
	}
	if apps := csvField(payload, "apps"); apps != "" {
		args = append(args, "--apps="+apps)
	}
	if staging := stringField(payload, "staging_id"); staging != "" {
		args = append(args, "--staging-id="+staging)
	}

	stdin := ""
	if raw, ok := payload["stdin_json"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			stdin = v
			if stdin != "" {
				args = append(args, "--payload-stdin")
			}
		default:
			return nil, "", fmt.Errorf("stdin_json must be a string")
		}
	}

	return args, stdin, nil
}

func legacyArgs(rawArgs []interface{}, payload map[string]interface{}) ([]string, string, error) {
	args := make([]string, 0, len(rawArgs))
	for _, item := range rawArgs {
		s, ok := item.(string)
		if !ok || s == "" {
			return nil, "", fmt.Errorf("payload.args must be strings")
		}
		args = append(args, s)
	}

	stdin := ""
	if raw, ok := payload["stdin_json"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			stdin = v
		default:
			return nil, "", fmt.Errorf("stdin_json must be a string")
		}
	}

	return args, stdin, nil
}

func stringField(payload map[string]interface{}, key string) string {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func boolField(payload map[string]interface{}, key string) bool {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return false
	}
	b, ok := raw.(bool)
	return ok && b
}

func csvField(payload map[string]interface{}, key string) string {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

func emitFailed(
	ctx context.Context,
	emit EventEmitter,
	cmd contract.Command,
	farmID string,
	step string,
	message string,
) error {
	return emit(ctx, contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        farmID,
		State:         "failed",
		Step:          step,
		Message:       message,
		Timestamp:     nowRFC3339(),
		EventType:     "progress",
	})
}

func progressEvent(
	cmd contract.Command,
	farmID string,
	state string,
	step string,
	message string,
	percent int,
) contract.Event {
	return contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        farmID,
		State:         state,
		Step:          step,
		Message:       message,
		Percent:       percent,
		Timestamp:     nowRFC3339(),
		EventType:     "progress",
	}
}
