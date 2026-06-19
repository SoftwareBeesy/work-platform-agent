package manage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// RunSync invokes manage without --async and parses JSON stdout when present.
func (a *Adapter) RunSync(ctx context.Context, args []string, stdin string) (map[string]any, error) {
	if a.BinPath == "" {
		return nil, fmt.Errorf("manage binary path is empty")
	}

	argv := append([]string{}, args...)
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
		return nil, fmt.Errorf("manage sync exec: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return map[string]any{"stdout": ""}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]any{"stdout": raw}, nil
	}

	return parsed, nil
}
