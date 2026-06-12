# Farm Agent — setup (Sprint N17)

Daemon **outbound-only** que conecta cada host de fazenda ao control plane (`work-platform-api`). Não abre portas inbound.

## Requisitos

- Go 1.22+ (build) ou binário release
- Ubuntu 24.04 (systemd)
- Token por `farm_id` emitido pelo control plane (Sprint N18)
- Certificados mTLS opcionais (`AGENT_TLS_*`)

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
|----------|-------------|-----------|
| `FARM_ID` | sim | Identificador da fazenda (`farm-saas-prod-01`) |
| `CONTROL_PLANE_URL` | sim | Base HTTPS do control plane (sem trailing slash) |
| `AGENT_TOKEN` | sim | Bearer token por fazenda |
| `AGENT_TLS_CERT` | não | Client cert PEM (mTLS) |
| `AGENT_TLS_KEY` | não | Client key PEM |
| `AGENT_TLS_CA` | não | CA para validar o control plane |
| `AGENT_QUEUE_PATH` | não | SQLite offline queue (default `/var/lib/mework360-platform-agent/events.db`) |
| `AGENT_POLL_TIMEOUT_SEC` | não | Long-poll timeout (default `55`) |
| `AGENT_HEARTBEAT_SEC` | não | Heartbeat interval (default `30`) |

## Build

```bash
go build -o /usr/local/bin/mework360-platform-agent ./cmd/agent
```

## systemd

```bash
sudo install -d -m 0750 -o root -g mework360 /var/lib/mework360-platform-agent
sudo cp deploy/mework360-platform-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now mework360-platform-agent
```

Arquivo de ambiente: `/etc/mework360/platform-agent.env` (modo `0600`).

## Protocolo (Fase 1a)

| Direção | Método | Path |
|---------|--------|------|
| Agente → CP | `GET` | `/api/agent/v1/commands?farm_id=&timeout=` (long-poll) |
| Agente → CP | `POST` | `/api/agent/v1/events` (heartbeat + progresso) |

Operação de teste: `agent.ping` → evento `pong` / `succeeded`.

Falha de rede: eventos enfileirados em SQLite e reenviados com backoff exponencial.

## Gate N17

1. Daemon sobe via systemd
2. Conexão outbound ao control plane (HTTPS + token; mTLS quando configurado)
3. Responde `agent.ping`
4. Zero portas inbound no host

Próximo: Sprint N18 (`AgentGateway` no `work-platform-api`).
