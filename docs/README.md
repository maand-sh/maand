# Maand documentation

Maand is an agentless workload orchestrator. State lives in a local **bucket** (`maand.db`, KV, secrets, staged files). Workers are reached over **SSH** from the **CLI host**; job commands run on the host with **python3** and **bun**.

Official site: [maand.sh/latest](https://maand.sh/latest)

---

## Start here

| Document | Description |
|----------|-------------|
| [start/overview.md](./start/overview.md) | What maand can do — strengths, limits, fit |
| [start/concepts.md](./start/concepts.md) | **Worker**, **job**, **allocation**, bucket layout |
| [start/quickstart.md](./start/quickstart.md) | Hands-on: init → workers → job → build → deploy |

---

## Guides (task-oriented)

| Document | When to read |
|----------|--------------|
| [guides/rolling-deploy.md](./guides/rolling-deploy.md) | Rolling upgrades, batch sizes, `deploy_order` |
| [guides/worker-reboot.md](./guides/worker-reboot.md) | Host OS reboot without losing catalog state |
| [guides/disable-and-drain.md](./guides/disable-and-drain.md) | Disable workers, jobs, or allocations |
| [guides/debugging-deploy.md](./guides/debugging-deploy.md) | Deploy skips, partial rollouts, dry-run |
| [guides/job-commands-tutorial.md](./guides/job-commands-tutorial.md) | Python/Bun hooks walkthrough |
| [guides/day-2-ops.md](./guides/day-2-ops.md) | Job control, health, GC patterns |
| [guides/prometheus.md](./guides/prometheus.md) | `_prometheus/` — scrape, alerts, runbooks, dashboards (optional per job) |

---

## Reference

### Job model

| Document | Topic |
|----------|-------|
| [reference/manifest.md](./reference/manifest.md) | **`manifest.json`** schema (canonical) |
| [reference/deployment-sequence.md](./reference/deployment-sequence.md) | Demands, **`deployment_seq`**, deploy waves |
| [reference/kv/README.md](./reference/kv/README.md) | KV index |
| [reference/kv/namespaces.md](./reference/kv/namespaces.md) | KV keys, layers, examples |
| [reference/kv/persistence.md](./reference/kv/persistence.md) | KV persistence, purge, access |
| [reference/job-command-api.md](./reference/job-command-api.md) | Runtime HTTP API, Python/Bun helpers |
| [reference/configuration.md](./reference/configuration.md) | `maand.conf`, `bucket.conf`, `vars.toml` |
| [reference/templates.md](./reference/templates.md) | `.tpl` rendering |
| [reference/certs.md](./reference/certs.md) | TLS CA and job certificates |
| [reference/resources-and-placement.md](./reference/resources-and-placement.md) | Memory, CPU, selectors |

### CLI commands

| Document | Command |
|----------|---------|
| [reference/cli/commands.md](./reference/cli/commands.md) | Full CLI index |
| [reference/cli/build.md](./reference/cli/build.md) | `maand build` |
| [reference/cli/deploy.md](./reference/cli/deploy.md) | `maand deploy` pipeline |
| [reference/cli/health-check.md](./reference/cli/health-check.md) | `maand health_check` |
| [reference/cli/job-command.md](./reference/cli/job-command.md) | Hook events and patterns |
| [reference/cli/job.md](./reference/cli/job.md) | `maand job` |
| [reference/cli/run-command.md](./reference/cli/run-command.md) | `maand run_command` |
| [reference/cli/gc.md](./reference/cli/gc.md) | `maand gc` |
| [reference/cli/info.md](./reference/cli/info.md) | `maand info` |

---

## Typical workflow

```text
maand init → edit workspace → maand build → maand deploy → health_check / job ops → gc
```

---

## Inspect state

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
maand cat deployments
maand cat certs
maand cat kv get maand/job/<job> version
```

Rollout debugging: [guides/debugging-deploy.md](./guides/debugging-deploy.md)

---

## Bucket layout

```text
./                          # bucket root
├── maand.conf
├── data/maand.db
├── workspace/
│   ├── workers.json
│   ├── disabled.json       # optional
│   ├── bucket.conf
│   └── jobs/<job>/
│       ├── manifest.json
│       ├── Makefile
│       ├── _modules/
│       ├── _prometheus/        # optional — see guides/prometheus.md
│       └── *.tpl
├── secrets/
├── tmp/
└── logs/
```

---

## Reader paths

| Goal | Path |
|------|------|
| First deploy | [quickstart](./start/quickstart.md) → [concepts](./start/concepts.md) |
| Stateful rolling cluster | [rolling-deploy](./guides/rolling-deploy.md) → [manifest](./reference/manifest.md) |
| Multi-job dependencies | [deployment-sequence](./reference/deployment-sequence.md) → [deploy](./reference/cli/deploy.md) |
| Something failed on deploy | [debugging-deploy](./guides/debugging-deploy.md) |
| Prometheus / metrics | [prometheus](./guides/prometheus.md) → [deploy](./reference/cli/deploy.md#prometheus-job-staging) |

---

## Local integration tests

See [assets/README.md](../assets/README.md), then:

```bash
go test -tags=integration ./tests/integration/... -v -timeout 25m
```

Not run in CI.
