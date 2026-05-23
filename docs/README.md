# Maand documentation

Maand is an agentless workload orchestrator. State lives in a local **bucket** directory (`maand.db`, KV store, secrets, and staged files). Workers are reached over **SSH** from the **maand CLI host**; job commands run on the host with **python3** and **bun**.

Official site: [maand.sh/latest](https://maand.sh/latest)

---

## Start here

| Document | Description |
|----------|-------------|
| [concepts.md](./concepts.md) | **Worker**, **job**, **allocation**, bucket layout, lifecycle |
| [jobs-and-dependencies.md](./jobs-and-dependencies.md) | **Job** manifest, **demands**, **`version`**, **`current_version`/`new_version`**, **`deployment_seq`**, deploy waves |
| [commands.md](./commands.md) | Full **command reference** |
| [tutorials/getting-started.md](./tutorials/getting-started.md) | Step-by-step: init → workers → job → build → deploy |
| [tutorials/day-2-operations.md](./tutorials/day-2-operations.md) | Job control, health checks, disable/remove, GC |
| [tutorials/job-commands.md](./tutorials/job-commands.md) | Python/Bun hooks and CLI commands |

---

## Typical workflow

```text
maand init          # once per bucket directory
# edit workspace/workers.json, workspace/jobs/*, workspace/maand.conf
maand build         # plan: workers, jobs, allocations, KV, certs
maand deploy        # push to workers, start/restart, hooks
maand health_check  # optional: run health_check commands
maand gc            # optional: purge removed allocations + old KV
```

---

## Command guides

| Document | Command | Role |
|----------|---------|------|
| [build.md](./build.md) | `maand build` | Read workspace → SQLite + KV; certs; `post_build` hooks |
| [jobs-and-dependencies.md](./jobs-and-dependencies.md) | Jobs & deps | Manifest, demands, **version**, allocation **current/new version**, `deployment_seq` |
| [deploy.md](./deploy.md) | `maand deploy` | Stage files, rsync, rollout, hooks, version env, `--dry-run` |
| [health-check.md](./health-check.md) | `maand health_check` | Run `health_check` job commands |
| [job-command.md](./job-command.md) | Job commands (all events) | Python/Bun scripts, runtime API, KV, demands |
| [job.md](./job.md) | `maand job` | Manual start/stop/restart/run/status |
| [run-command.md](./run-command.md) | `maand run_command` | Ad-hoc SSH commands on workers |
| [gc.md](./gc.md) | `maand gc` | Purge removed allocations and old KV |
| [info.md](./info.md) | `maand info` | Bucket summary |

---

## Inspect state

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
maand cat job_commands
maand cat job_ports
maand cat kv
maand cat kv get <namespace> <key>
```

---

## Bucket layout (after `maand init`)

```text
./                          # bucket root (bucket.Location)
├── maand.conf              # SSH user, key, sudo, cert TTL, job_config_selector
├── data/maand.db           # SQLite catalog + KV history
├── workspace/
│   ├── workers.json
│   ├── disabled.json       # optional
│   ├── bucket.conf         # port pool (port_min/port_max) + vars → KV
│   └── jobs/<job>/
│       ├── manifest.json
│       ├── Makefile        # required (start/stop/restart for default deploy)
│       ├── _modules/       # command_<name>.py | .ts | .js
│       └── *.tpl           # optional templates
├── secrets/                # CA, worker SSH key, kv.key
├── tmp/                    # staging for deploy / job commands
└── logs/                   # runtime API logs
```

---

## Local integration tests (real workers)

Copy [assets/README.md](../assets/README.md) setup, then:

```bash
go test -tags=integration ./tests/integration/... -v -timeout 25m
```

Not run in CI.
