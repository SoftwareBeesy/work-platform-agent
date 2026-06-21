package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/manage"
)

// EventEmitter posts progress events to the control plane.
type EventEmitter func(ctx context.Context, event contract.Event) error

// HandleCustomAppsUpdate dispatches custom-apps update via manage CLI.
func HandleCustomAppsUpdate(
	ctx context.Context,
	cmd contract.Command,
	farmID string,
	invoker manage.Invoker,
	emit EventEmitter,
) error {
	args, err := buildCustomAppsUpdateArgs(cmd.Payload)
	if err != nil {
		return emit(ctx, progressEvent(cmd, farmID, "failed", "validate", err.Error(), 0, nil))
	}

	parsed, err := invoker.RunAsync(ctx, args, "")
	if err != nil {
		return emit(ctx, progressEvent(cmd, farmID, "failed", "manage_exec", err.Error(), 0, nil))
	}

	jobID, err := manage.JobID(parsed)
	if err != nil {
		return emit(ctx, progressEvent(cmd, farmID, "failed", "parse_job_id", err.Error(), 0, nil))
	}

	return emit(ctx, progressEvent(cmd, farmID, "succeeded", "dispatched", "job queued on farm", 100, map[string]any{
		"job_id": jobID,
	}))
}

func buildCustomAppsUpdateArgs(payload map[string]interface{}) ([]string, error) {
	ring := payloadString(payload, "ring")
	tenant := payloadString(payload, "tenant")
	appID := payloadString(payload, "app_id")
	wantJSON := payloadBool(payload, "json")

	hasRing := ring != ""
	hasTenant := tenant != ""

	if !hasRing && !hasTenant {
		return nil, fmt.Errorf("either ring or tenant is required")
	}
	if hasRing && hasTenant {
		return nil, fmt.Errorf("ring and tenant are mutually exclusive")
	}

	if hasRing {
		ring = strings.ToLower(ring)
		if ring != "canary" && ring != "stable" {
			return nil, fmt.Errorf("ring must be canary or stable")
		}
	}

	args := []string{"custom-apps", "update"}
	if hasRing {
		args = append(args, "--ring="+ring)
	} else {
		args = append(args, "--tenant="+tenant)
	}
	if appID != "" {
		args = append(args, "--app="+appID)
	}
	if wantJSON {
		args = append(args, "--json")
	}

	return args, nil
}

func payloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
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

func payloadBool(payload map[string]interface{}, key string) bool {
	if payload == nil {
		return false
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return false
	}
	b, ok := raw.(bool)
	return ok && b
}

func progressEvent(
	cmd contract.Command,
	farmID, state, step, message string,
	percent int,
	data map[string]any,
) contract.Event {
	return contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        farmID,
		State:         state,
		Step:          step,
		Message:       message,
		Percent:       percent,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		EventType:     "progress",
		Data:          data,
	}
}
