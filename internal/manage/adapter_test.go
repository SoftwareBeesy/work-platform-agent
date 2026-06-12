package manage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunAsyncAppendsAsyncJsonFlags(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper not used on windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "nextcloud-manage.sh")
	content := `#!/bin/sh
echo "$@" >&2
printf '%s' '{"job_id":"job-test-1"}'
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter(script)
	parsed, err := adapter.RunAsync(context.Background(), []string{"acme", "acme.example", "create"}, "")
	if err != nil {
		t.Fatalf("RunAsync: %v", err)
	}

	jobID, err := JobID(parsed)
	if err != nil {
		t.Fatalf("JobID: %v", err)
	}
	if jobID != "job-test-1" {
		t.Fatalf("job_id = %q", jobID)
	}
}

func TestJobIDMissing(t *testing.T) {
	t.Parallel()
	_, err := JobID(map[string]any{"state": "queued"})
	if err == nil {
		t.Fatal("expected error")
	}
}
