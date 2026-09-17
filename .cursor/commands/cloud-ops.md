# beesy-project-command-shim
# cloud-ops

> **Hub**: WHMCS + Proxmox mecloud360 — provisionar, operar, drift, ship/rollback.
> **Aliases**: `/cloud-ops`, `/cloud-ops consultar`, `/cloud-ops provisionar`, `/cloud-ops operar`, `/cloud-ops diagnosticar`, `/cloud-ops auditar`, `/cloud-ops publicar`, `/cloud-ops gc`
> **Shim**: instalado por `scripts/install-project-commands.sh` — skill canônica em `~/.cursor/skills/cloud-ops/`.

Operações cloud mecloud360 via WHMCS API (escrita) e Proxmox read-only (drift/diagnóstico). Política #233: escrita sempre via WHMCS.

---

## Ajuda (?)

Se o usuário digitar `/cloud-ops ?`, exibir o help menu e **NÃO** executar nada.

```
/cloud-ops ?  →  carregar references/help-menu.md da skill cloud-ops
```

---

## Skills

1. Leia `~/.cursor/skills/cloud-ops/SKILL.md`
2. Siga o procedimento da skill obrigatoriamente
3. Modos e políticas: `~/.cursor/skills/cloud-ops/references/help-menu.md`

## Uso

```
/cloud-ops ?                              → menu de modos
/cloud-ops consultar [pergunta]           → leitura WHMCS/Proxmox (zero escrita)
/cloud-ops provisionar [produto|pid]      → novo pedido via WHMCS
/cloud-ops operar [ação] [service-id]     → ciclo de vida via WHMCS API
/cloud-ops diagnosticar [cluster|node]      → saúde Proxmox read-only
/cloud-ops auditar [drift|hardening]      → drift WHMCS↔Proxmox
/cloud-ops publicar [homolog|dokploy]     → ship via scripts auditados
/cloud-ops rollback homolog               → rollback homolog
/cloud-ops gc                             → limpeza /tmp/beesy-* via cloud-gc.sh
```

## Regras

- Responda sempre em português brasileiro
- Credenciais: modo **client** → `~/.config/beesy/credentials.json`; modo **admin** → `~/.config/beesy/cloud.env` via `beesy-secrets materialize`
- Nunca exponha valores de secrets em chat, commits ou logs
- Ship/rollback/gc: somente `scripts/cloud-ship.sh` e `scripts/cloud-gc.sh`
