package queue

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite persists outbound events when the control plane is unreachable.
type SQLite struct {
	db *sql.DB
}

// Open creates or opens the offline event queue database.
func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite queue: %w", err)
	}
	q := &SQLite{db: db}
	if err := q.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return q, nil
}

func (q *SQLite) migrate() error {
	_, err := q.db.Exec(`
		CREATE TABLE IF NOT EXISTS outbound_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payload TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate queue: %w", err)
	}
	return nil
}

// Enqueue stores a JSON event payload for later delivery.
func (q *SQLite) Enqueue(ctx context.Context, payload string) error {
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO outbound_events (payload, created_at, attempts) VALUES (?, ?, 0)`,
		payload, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}
	return nil
}

// Row represents a queued outbound event.
type Row struct {
	ID      int64
	Payload string
}

// DequeueAll returns queued rows in FIFO order without deleting them.
func (q *SQLite) DequeueAll(ctx context.Context) ([]Row, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id, payload FROM outbound_events ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("dequeue all: %w", err)
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.ID, &row.Payload); err != nil {
			return nil, fmt.Errorf("scan queued row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queued rows: %w", err)
	}
	return out, nil
}

// Delete removes a delivered event by id.
func (q *SQLite) Delete(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM outbound_events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete queued event: %w", err)
	}
	return nil
}

// BumpAttempts increments retry counter for a queued event.
func (q *SQLite) BumpAttempts(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE outbound_events SET attempts = attempts + 1 WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("bump attempts: %w", err)
	}
	return nil
}

// Close closes the underlying database.
func (q *SQLite) Close() error {
	return q.db.Close()
}
