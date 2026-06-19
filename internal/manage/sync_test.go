package manage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunSyncParsesJSON(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper not used on windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "nextcloud-manage.sh")
	content := `#!/bin/sh
printf '%s' '{"value":"configured"}'
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter(script)
	parsed, err := adapter.RunSync(context.Background(), []string{"acme", "occ-exec", "config:app:get", "mework360_memail", "forceSSO"}, "")
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if parsed["value"] != "configured" {
		t.Fatalf("value = %v", parsed["value"])
	}
}
