# Maand documentation

Maand is an agentless workload orchestrator. State lives in a local **bucket** directory (`maand.db`, KV store, secrets, and staged files). Workers are reached over **SSH** from the **maand CLI host**; job commands run on the host with **python3** and **bun**.

Official site: [maand.sh/latest](https://maand.sh/latest)

---

## Start here

| Document | Description |
|----------|-------------|
| [capabilities.md](./capabilities.md) | What maand can do — strengths, limits, and fit |
| [concepts.md](./concepts.md) | **Worker**, **job**, **allocation**, bucket layout, lifecycle |
| [tutorials/getting-started.md](./tutorials/getting-started.md) | Hands-on: init → workers → job → build → deploy |

---

## Tutorials

| Document | Description |
|----------|-------------|
| [tutorials/getting-started.md](./tutorials/getting-started.md) | First bucket and deploy |
| [tutorials/day-2-operations.md](./tutorials/day-2-operations.md) | Job control, health checks, disable/remove, GC |
| [tutorials/job-commands.md](./tutorials/job-commands.md) | Python/Bun hooks and CLI commands |

---

## Command reference

| Document | Command / topic |
|----------|-----------------|
| [commands.md](./commands.md) | Full CLI reference (all commands and flags) |
| [configuration.md](./configuration.md) | `maand.conf`, `bucket.conf`, `bucket.jobs.conf`, `vars.toml` |
| [resources-and-placement.md](./resources-and-placement.md) | Job memory/CPU, bucket overrides, environment selectors |
| [build.md](./build.md) | `maand build` |
| [deploy.md](./deploy.md) | `maand deploy` |
| [health-check.md](./health-check.md) | `maand health_check` |
| [job.md](./job.md) | `maand job` (start/stop/restart/run/status/create) |
| [job-command.md](./job-command.md) | `maand job_command` and hook events |
| [run-command.md](./run-command.md) | `maand run_command` |
| [gc.md](./gc.md) | `maand gc` |
| [info.md](./info.md) | `maand info` |

---

## Job and deploy model

| Document | Topic |
|----------|-------|
| [jobs-and-dependencies.md](./jobs-and-dependencies.md) | Manifest, **demands**, **version**, **`deployment_seq`**, deploy waves |
| [kv.md](./kv.md) | KV namespaces, secrets, persistence |
| [templates.md](./templates.md) | `.tpl` rendering at deploy |
| [rolling-upgrade.md](./rolling-upgrade.md) | Rolling job upgrades and worker reboots |
| [disabled.md](./disabled.md) | Disable/re-enable workers, jobs, allocations |
| [deploy-debugging.md](./deploy-debugging.md) | Troubleshoot deploy skips, failures, partial rollouts |

---

## Typical workflow

```text
maand init          # once per bucket directory
# edit workspace/workers.json, workspace/jobs/*, maand.conf
maand build         # catalog + KV + certs; post_build hooks
maand deploy        # rsync + start/restart + hooks
maand health_check  # optional: worker + job health
maand gc            # optional: purge removed allocations + old KV
```

---

## Inspect state

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
maand cat hashes
maand cat job_commands
maand cat job_ports
maand cat kv
maand cat kv get <namespace> <key>
maand cat kv get --reveal secrets/job/<job> <key>
```

Rollout debugging: [deploy-debugging.md](./deploy-debugging.md) · Hash columns: [commands.md](./commands.md#maand-cat-hashes)

---

## Bucket layout (after `maand init`)

```text
./                          # bucket root
├── maand.conf              # SSH, certs, job config selector — see configuration.md
├── data/maand.db           # SQLite catalog + KV history
├── workspace/
│   ├── workers.json
│   ├── disabled.json       # optional
│   ├── bucket.conf         # port pool + vars → KV
│   └── jobs/<job>/
│       ├── manifest.json
│       ├── Makefile        # start/stop/restart (default deploy)
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

Not run in CI (GitHub Actions runs `make test-unit` and `make build` only).
