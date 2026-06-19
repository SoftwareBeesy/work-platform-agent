package occ

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/manage"
)

const (
	defaultTimeout = 120 * time.Second
	maxRetries     = 3
)

// SyncInvoker runs nextcloud-manage synchronously (no --async).
type SyncInvoker interface {
	RunSync(ctx context.Context, args []string, stdin string) (map[string]any, error)
}

// Adapter executes allowlisted occ-exec subcommands with timeout, retry, and JSON parse.
type Adapter struct {
	manage    SyncInvoker
	timeout   time.Duration
	maxRetry  int
}

// NewAdapter returns an OccAdapter backed by the manage sync invoker.
func NewAdapter(invoker SyncInvoker) *Adapter {
	return &Adapter{
		manage:   invoker,
		timeout:  defaultTimeout,
		maxRetry: maxRetries,
	}
}

// Exec runs `<slug> occ-exec <subcmd> [args...] --json` with retry on transient errors.
func (a *Adapter) Exec(
	ctx context.Context,
	tenantSlug string,
	subcmd string,
	args []string,
) (map[string]any, error) {
	if strings.TrimSpace(tenantSlug) == "" {
		return nil, fmt.Errorf("tenant slug is required")
	}
	if strings.TrimSpace(subcmd) == "" {
		return nil, fmt.Errorf("occ subcmd is required")
	}

	argv := append([]string{tenantSlug, "occ-exec", subcmd}, args...)
	argv = append(argv, "--json")

	var lastErr error
	for attempt := 0; attempt < a.maxRetry; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, a.timeout)
		parsed, err := a.manage.RunSync(attemptCtx, argv, "")
		cancel()
		if err == nil {
			return parsed, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == a.maxRetry-1 {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}

	return nil, lastErr
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "retryable") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "exit code 2")
}

// Ensure manage.Adapter satisfies SyncInvoker when RunSync is added.
var _ SyncInvoker = (*manage.Adapter)(nil)
