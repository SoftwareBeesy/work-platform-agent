package manage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Invoker runs nextcloud-manage locally on the farm host.
type Invoker interface {
	RunAsync(ctx context.Context, args []string, stdin string) (map[string]any, error)
}

// Adapter executes the manage CLI with --async --json appended.
type Adapter struct {
	BinPath string
}

// NewAdapter returns an invoker for the given binary path.
func NewAdapter(binPath string) *Adapter {
	return &Adapter{BinPath: strings.TrimSpace(binPath)}
}

// RunAsync invokes manage and parses the JSON stdout for job_id.
func (a *Adapter) RunAsync(ctx context.Context, args []string, stdin string) (map[string]any, error) {
	if a.BinPath == "" {
		return nil, fmt.Errorf("manage binary path is empty")
	}

	argv := append([]string{}, args...)
	if !hasFlag(argv, "--async") {
		argv = append(argv, "--async")
	}
	if !hasFlag(argv, "--json") {
		argv = append(argv, "--json")
	}

	cmd := exec.CommandContext(ctx, a.BinPath, argv...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("manage exec: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return nil, fmt.Errorf("manage returned empty stdout")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse manage json: %w", err)
	}

	return parsed, nil
}

// JobID extracts job_id from manage JSON response.
func JobID(parsed map[string]any) (string, error) {
	raw, ok := parsed["job_id"]
	if !ok {
		return "", fmt.Errorf("manage response missing job_id")
	}
	jobID, ok := raw.(string)
	if !ok || jobID == "" {
		return "", fmt.Errorf("manage response job_id invalid")
	}
	return jobID, nil
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

// DefaultBinPath resolves NEXTCLOUD_MANAGE_PATH or a conventional default.
func DefaultBinPath() string {
	if path := strings.TrimSpace(os.Getenv("NEXTCLOUD_MANAGE_PATH")); path != "" {
		return path
	}
	return "/usr/local/bin/nextcloud-manage"
}
