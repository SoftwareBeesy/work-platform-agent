# work-platform-agent

Farm Agent daemon for Platform V2 — outbound HTTPS/mTLS transport from each Nextcloud farm host to the control plane (`work-platform-api`).

## Sprint N17 scope

- Go 1.22 scaffold + CI
- mTLS-ready HTTP client + bearer token per `farm_id`
- Long-poll `GET /api/agent/v1/commands`
- `POST /api/agent/v1/events` (heartbeat + progress)
- SQLite offline queue + exponential backoff
- `agent.ping` handler (scaffold)

See [`docs/AGENT-SETUP.md`](docs/AGENT-SETUP.md) and [`work-platform-api/docs/PLATFORM-V2-PLAN.md`](../work-platform-api/docs/PLATFORM-V2-PLAN.md) §10.

## Quick start (dev)

```bash
export FARM_ID=farm-dev-01
export CONTROL_PLANE_URL=https://localhost:8443
export AGENT_TOKEN=dev-token
export AGENT_QUEUE_PATH=/tmp/agent-events.db
go run ./cmd/agent
```

## Test

```bash
go test ./... -race
```
