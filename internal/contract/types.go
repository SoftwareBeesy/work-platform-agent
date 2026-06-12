package contract

import "time"

// Command is a control-plane instruction delivered to the farm agent.
type Command struct {
	SchemaVersion  int                    `json:"schema_version"`
	OperationID    string                 `json:"operation_id"`
	Operation      string                 `json:"operation"`
	FarmID         string                 `json:"farm_id"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	RequestedAt    time.Time              `json:"requested_at,omitempty"`
}

// CommandsResponse wraps zero or more commands from long-poll.
type CommandsResponse struct {
	SchemaVersion int       `json:"schema_version"`
	Commands      []Command `json:"commands"`
}

// Event is progress or heartbeat emitted by the agent.
type Event struct {
	OperationID   string         `json:"operation_id,omitempty"`
	FarmID        string         `json:"farm_id"`
	State         string         `json:"state"`
	Step          string         `json:"step,omitempty"`
	Message       string         `json:"message,omitempty"`
	Percent       int            `json:"percent,omitempty"`
	Timestamp     string         `json:"ts"`
	SchemaVersion int            `json:"schema_version"`
	EventType     string         `json:"event_type,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
}
