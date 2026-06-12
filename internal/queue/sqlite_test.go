package queue_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/SoftwareBeesy/work-platform-agent/internal/queue"
)

func TestSQLiteEnqueueDequeueDelete(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "events.db")
	q, err := queue.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer q.Close()

	ctx := context.Background()
	if err := q.Enqueue(ctx, `{"state":"running"}`); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	rows, err := q.DequeueAll(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if err := q.Delete(ctx, rows[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rows, err = q.DequeueAll(ctx)
	if err != nil {
		t.Fatalf("dequeue after delete: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty queue, got %d", len(rows))
	}
}
