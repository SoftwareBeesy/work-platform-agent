package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/config"
	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/manage"
	"github.com/SoftwareBeesy/work-platform-agent/internal/occ"
	"github.com/SoftwareBeesy/work-platform-agent/internal/ops"
	"github.com/SoftwareBeesy/work-platform-agent/internal/queue"
	"github.com/SoftwareBeesy/work-platform-agent/internal/transport"
)

// Transport abstracts control-plane HTTP calls for tests.
type Transport interface {
	PollCommands(ctx context.Context, timeout time.Duration) ([]contract.Command, error)
	PostEvent(ctx context.Context, event contract.Event) error
}

// EventStore abstracts the offline queue for tests.
type EventStore interface {
	Enqueue(ctx context.Context, payload string) error
	DequeueAll(ctx context.Context) ([]queue.Row, error)
	Delete(ctx context.Context, id int64) error
	BumpAttempts(ctx context.Context, id int64) error
}

// Runner is the main daemon loop (poll, handle, heartbeat, flush queue).
type Runner struct {
	cfg       config.Config
	transport Transport
	queue     EventStore
	manage    manage.Invoker
	occ       *occ.Adapter
	logger    *slog.Logger
	backoff   time.Duration
}

// NewRunner wires dependencies for the daemon loop.
func NewRunner(cfg config.Config, tp Transport, q EventStore, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	manageAdapter := manage.NewAdapter(cfg.ManageBinPath)
	return &Runner{
		cfg:       cfg,
		transport: tp,
		queue:     q,
		manage:    manageAdapter,
		occ:       occ.NewAdapter(manageAdapter),
		logger:    logger,
		backoff:   cfg.InitialBackoff,
	}
}

// Run executes until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	heartbeatTicker := time.NewTicker(r.cfg.HeartbeatEvery)
	defer heartbeatTicker.Stop()

	for {
		if err := r.flushQueue(ctx); err != nil {
			r.logger.Warn("flush queue failed", "err", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeatTicker.C:
			if err := r.sendHeartbeat(ctx); err != nil {
				r.logger.Warn("heartbeat failed", "err", err)
				r.sleepBackoff(ctx)
			} else {
				r.resetBackoff()
			}
		default:
		}

		commands, err := r.transport.PollCommands(ctx, r.cfg.PollTimeout)
		if err != nil {
			r.logger.Warn("poll failed", "err", err)
			r.sleepBackoff(ctx)
			continue
		}
		r.resetBackoff()

		for _, cmd := range commands {
			if err := r.handleCommand(ctx, cmd); err != nil {
				r.logger.Error("handle command failed", "operation", cmd.Operation, "err", err)
			}
		}

		if len(commands) == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func (r *Runner) handleCommand(ctx context.Context, cmd contract.Command) error {
	switch cmd.Operation {
	case "agent.ping", "ping":
		event := contract.Event{
			SchemaVersion: 1,
			OperationID:   cmd.OperationID,
			FarmID:        r.cfg.FarmID,
			State:         "succeeded",
			Step:          "pong",
			Message:       "pong",
			Percent:       100,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			EventType:     "progress",
		}
		return r.emitEvent(ctx, event)
	case "tenant.create":
		return ops.HandleTenantCreate(ctx, cmd, r.cfg.FarmID, r.manage, r.emitEvent)
	case "memail.configure":
		return ops.HandleMemailConfigure(ctx, cmd, r.cfg.FarmID, r.occ, r.emitEvent)
	case "tenant.remove":
		return r.handleManageOperation(ctx, cmd)
	default:
		event := contract.Event{
			SchemaVersion: 1,
			OperationID:   cmd.OperationID,
			FarmID:        r.cfg.FarmID,
			State:         "failed",
			Step:          "unsupported",
			Message:       fmt.Sprintf("operation %q not implemented in N17 scaffold", cmd.Operation),
			Percent:       0,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			EventType:     "progress",
		}
		return r.emitEvent(ctx, event)
	}
}

func (r *Runner) handleManageOperation(ctx context.Context, cmd contract.Command) error {
	args, stdin, err := manageArgsFromPayload(cmd.Payload)
	if err != nil {
		return r.emitEvent(ctx, contract.Event{
			SchemaVersion: 1,
			OperationID:   cmd.OperationID,
			FarmID:        r.cfg.FarmID,
			State:         "failed",
			Step:          "validate",
			Message:       err.Error(),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			EventType:     "progress",
		})
	}

	parsed, err := r.manage.RunAsync(ctx, args, stdin)
	if err != nil {
		return r.emitEvent(ctx, contract.Event{
			SchemaVersion: 1,
			OperationID:   cmd.OperationID,
			FarmID:        r.cfg.FarmID,
			State:         "failed",
			Step:          "manage_exec",
			Message:       err.Error(),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			EventType:     "progress",
		})
	}

	jobID, err := manage.JobID(parsed)
	if err != nil {
		return r.emitEvent(ctx, contract.Event{
			SchemaVersion: 1,
			OperationID:   cmd.OperationID,
			FarmID:        r.cfg.FarmID,
			State:         "failed",
			Step:          "parse_job_id",
			Message:       err.Error(),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			EventType:     "progress",
		})
	}

	return r.emitEvent(ctx, contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        r.cfg.FarmID,
		State:         "succeeded",
		Step:          "dispatched",
		Message:       "job queued on farm",
		Percent:       100,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		EventType:     "progress",
		Data: map[string]any{
			"job_id": jobID,
		},
	})
}

func manageArgsFromPayload(payload map[string]interface{}) ([]string, string, error) {
	if payload == nil {
		return nil, "", fmt.Errorf("payload is required")
	}

	rawArgs, ok := payload["args"].([]interface{})
	if !ok || len(rawArgs) == 0 {
		return nil, "", fmt.Errorf("payload.args is required")
	}

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
			return nil, "", fmt.Errorf("payload.stdin_json must be a string")
		}
	}

	return args, stdin, nil
}

func (r *Runner) sendHeartbeat(ctx context.Context) error {
	event := contract.Event{
		SchemaVersion: 1,
		FarmID:        r.cfg.FarmID,
		State:         "running",
		Step:          "heartbeat",
		Message:       "agent alive",
		Percent:       0,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		EventType:     "heartbeat",
	}
	return r.emitEvent(ctx, event)
}

func (r *Runner) emitEvent(ctx context.Context, event contract.Event) error {
	if err := r.transport.PostEvent(ctx, event); err != nil {
		payload, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return fmt.Errorf("post event: %w (marshal queued payload: %v)", err, marshalErr)
		}
		if queueErr := r.queue.Enqueue(ctx, string(payload)); queueErr != nil {
			return fmt.Errorf("post event: %w (enqueue: %v)", err, queueErr)
		}
		return nil
	}
	return nil
}

func (r *Runner) flushQueue(ctx context.Context) error {
	rows, err := r.queue.DequeueAll(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var event contract.Event
		if err := json.Unmarshal([]byte(row.Payload), &event); err != nil {
			r.logger.Error("drop invalid queued event", "id", row.ID, "err", err)
			_ = r.queue.Delete(ctx, row.ID)
			continue
		}
		if err := r.transport.PostEvent(ctx, event); err != nil {
			_ = r.queue.BumpAttempts(ctx, row.ID)
			return err
		}
		if err := r.queue.Delete(ctx, row.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) sleepBackoff(ctx context.Context) {
	timer := time.NewTimer(r.backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	r.backoff *= 2
	if r.backoff > r.cfg.MaxBackoff {
		r.backoff = r.cfg.MaxBackoff
	}
}

func (r *Runner) resetBackoff() {
	r.backoff = r.cfg.InitialBackoff
}

// Ensure Runner uses transport.Client in production builds.
var _ Transport = (*transport.Client)(nil)
