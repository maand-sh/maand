# Reference

Canonical technical documentation. Guides link here; avoid duplicating full schemas in guides.

## Configuration and job model

| Document | Topic |
|----------|-------|
| [configuration.md](./configuration.md) | `maand.conf`, `bucket.conf`, `bucket.jobs*.conf`, `vars.toml`, prerequisites |
| [manifest.md](./manifest.md) | `manifest.json` schema |
| [resources-and-placement.md](./resources-and-placement.md) | Memory, CPU bounds vs reservations, selectors, worker capacity |
| [deployment-sequence.md](./deployment-sequence.md) | Demands, `deployment_seq`, deploy waves |
| [templates.md](./templates.md) | `.tpl` rendering at deploy |
| [certs.md](./certs.md) | TLS CA and job certificates |

## KV and job commands

| Document | Topic |
|----------|-------|
| [kv/README.md](./kv/README.md) | KV index |
| [kv/namespaces.md](./kv/namespaces.md) | Keys, layers, examples |
| [kv/persistence.md](./kv/persistence.md) | Writes, purge, access rules |
| [job-command-api.md](./job-command-api.md) | Runtime HTTP API, env vars, Python/Bun helpers |

## Observability

| Document | Topic |
|----------|-------|
| [observability/logging.md](./observability/logging.md) | Log files, formats, `maand logs show`, terminal output |

## CLI

| Document | Command / topic |
|----------|-----------------|
| [cli/commands.md](./cli/commands.md) | Full CLI index |
| [cli/build.md](./cli/build.md) | `maand build` |
| [cli/deploy.md](./cli/deploy.md) | `maand deploy` |
| [cli/health-check.md](./cli/health-check.md) | `maand health_check` |
| [cli/job-command.md](./cli/job-command.md) | Hook events, `maand jobcommand` |
| [cli/job.md](./cli/job.md) | `maand job` |
| [cli/run-command.md](./cli/run-command.md) | `maand run_command` |
| [cli/gc.md](./cli/gc.md) | `maand gc` |
| [cli/info.md](./cli/info.md) | `maand info`, `maand cat` |
