# Command reference

Run `maand` from the **bucket root** (the directory created by `maand init`). Use `maand <command> --help` for flags.

Configuration files: [configuration.md](./configuration.md) · KV: [kv.md](./kv.md) · Templates: [templates.md](./templates.md)

## Workflow commands

| Command | Summary | Details |
|---------|---------|---------|
| `maand init` | Create or upgrade bucket (DB, workspace layout, CA, secrets) | — |
| `maand build` | Read workspace → update `maand.db`, KV, certs; run `post_build` hooks | [build.md](./build.md) |
| `maand deploy` | Push jobs to workers, roll out, run deploy hooks | [deploy.md](./deploy.md) · [rolling-upgrade.md](./rolling-upgrade.md) · [deploy-debugging.md](./deploy-debugging.md) |
| `maand health_check` | Worker SSH gate + per-job health (manifest probes or commands) | [health-check.md](./health-check.md) |
| `maand gc` | Purge removed allocations, worker data, old KV history | [gc.md](./gc.md) |

## Inspect commands

| Command | Summary |
|---------|---------|
| `maand info` | Bucket ID, update sequence, counts | [info.md](./info.md) |
| `maand cat workers` | Worker catalog |
| `maand cat jobs` | Job catalog (includes **`deployment_seq`**) |
| `maand cat allocations` | Job × worker rows (`--jobs`, `--workers` filters) |
| `maand cat deployments` | Allocation `current_hash` / `previous_hash` and rollout state (`--jobs`, `--workers`) |
| `maand cat job_commands` | Commands from manifests |
| `maand cat job_ports` | Declared ports per job |
| `maand cat kv` | List KV keys (`--jobs`, `--active`, `--deleted`; or `maand cat kv get <ns> <key> [--reveal]`) |

## Job control

Manual lifecycle on workers (requires prior deploy; validates `worker.json` / `update_seq`).

| Command | Summary | Details |
|---------|---------|---------|
| `maand job start <job>` | `make start` (or `job_control` command) | [job.md](./job.md) |
| `maand job stop <job>` | `make stop` | [job.md](./job.md) |
| `maand job restart <job>` | `make restart` | [job.md](./job.md) |
| `maand job run <job> --target <name>` | Arbitrary Makefile target | [job.md](./job.md) |
| `maand job status <job>` | `make status` | [job.md](./job.md) |
| `maand job create <job>` | Scaffold `workspace/jobs/<job>/` | [job.md](./job.md) |

Common flags: `--allocations ip,...`, `--health_check` (start/stop/restart/run).

## Job commands (hooks & CLI)

| Command | Summary | Details |
|---------|---------|---------|
| `maand job_command <job> <command>` | Run one manifest command with event **`cli`** | [job-command.md](./job-command.md) |

Flags: `--concurrency N`, `--verbose`.

The same commands run automatically on events: `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`.

## Ad-hoc worker shell

| Command | Summary | Details |
|---------|---------|---------|
| `maand run_command "<shell>"` | Run a command on workers over SSH | [run-command.md](./run-command.md) |

Flags: `--workers`, `--labels`, `--concurrency`, `--health_check`.

---

## `maand init`

```bash
maand init
```

Creates (first run) or upgrades (later runs):

- `data/maand.db` with schema migrations
- `workspace/workers.json` (empty `[]`), `workspace/jobs/`, `workspace/bucket.conf`
- `maand.conf` defaults — see [configuration.md](./configuration.md#maandconf-bucket-root)
- Bucket CA in `secrets/ca.crt` / `ca.key`
- KV encryption key `secrets/kv.key`
- `tmp/` and `logs/` directories

Does not contact workers. Re-running **`maand init`** on an existing bucket applies schema upgrades without changing **`bucket_id`** or the CA.

Other commands check the database schema before running. If the binary is newer than **`maand.db`**, they fail with a hint to run **`maand init`**.

---

## `maand build`

```bash
maand build [--purge-job-kv]
```

| Flag | Description |
|------|-------------|
| `--purge-job-kv` | Mark `vars/job/<job>` and `secrets/job/<job>` deleted when a job has no active allocations |

Reconciles the entire workspace. No filters. Validates job **demands** and **version** constraints. See [build.md](./build.md#job-version).

**Host prerequisites:** `python3`, `bun` (if TS/JS commands), for `post_build` hooks.

---

## `maand deploy`

```bash
maand deploy [--build] [--jobs j1,j2] [--dry-run] [--force]
```

| Flag | Description |
|------|-------------|
| `-b`, `--build` | Run `maand build` first |
| `--jobs` | Limit to named jobs (still respects deployment sequence) |
| `-n`, `--dry-run` | Compare allocation hashes; no worker changes |
| `--force` | Redeploy even when all allocations are already promoted |

**Host prerequisites:** `bash`, `ssh`, `rsync`, `python3`, optional `bun`.  
**Worker prerequisites:** `python3`, `make`, `rsync`, `bash`, `timeout`, optional `sudo`.

See [deploy.md](./deploy.md).

---

## `maand job`

```bash
maand job start|stop|restart|status <job> [--allocations ip,...] [--health_check]
maand job run <job> --target <make-target> [--allocations ip,...] [--health_check]
maand job create <job> [--selectors s1,s2]
```

See [job.md](./job.md).

---

## `maand job_command`

```bash
maand job_command <job> <command_name> [--concurrency N] [--verbose]
```

Command must include **`cli`** in manifest `executed_on`. Script: `workspace/jobs/<job>/_modules/command_<name>.py` (or `.ts`/`.js`).

**Host prerequisites:** `python3` or job venv; `bun` for TS/JS.

See [job-command.md](./job-command.md).

---

## `maand health_check`

```bash
maand health_check [--jobs j1,j2] [--wait] [--verbose] [--update-hash]
```

| Flag | Description |
|------|-------------|
| `--jobs` | Limit to named jobs |
| `--wait` | Retry until pass (up to 30 attempts per job) |
| `--verbose` | Stream command output |
| `--update-hash` | Mark failed allocations for redeploy (command-based health only) |

See [health-check.md](./health-check.md).

---

## `maand run_command`

```bash
maand run_command "<command>" [-w ip,...] [-l label,...] [-c N] [--health_check]
```

**Host prerequisites:** `bash`, `ssh`.  
**Worker prerequisites:** `bash`, `timeout`, optional `sudo`.

See [run-command.md](./run-command.md).

---

## `maand gc`

```bash
maand gc [--retain-days N]
```

See [gc.md](./gc.md).

---

## `maand info` and `maand cat`

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations [--jobs api] [--workers 10.0.0.1]
maand cat deployments [--jobs vault] [--workers 10.0.0.1]
maand cat job_commands
maand cat job_ports
maand cat kv
maand cat kv --jobs vault
maand cat kv --jobs vault --active
maand cat kv --deleted
maand cat kv get maand/job/api name
maand cat kv get --reveal secrets/job/vault root_token
```

See [info.md](./info.md).

### `maand cat deployments`

```bash
maand cat deployments [--jobs j1,j2] [--workers ip,...] [--active]
```

| Flag | Description |
|------|-------------|
| `--jobs` | Comma-separated job names |
| `--workers` | Comma-separated worker IPs |
| `--active` | Only active allocations (`removed=0`, `disabled=0`) |

Shows `current_hash`, `previous_hash`, versions, and rollout state per allocation. **Rollout** is `removed`, `disabled`, or `disabled_restart` when the allocation flag applies; otherwise hash/version state (`new`, `restart`, `promoted`, `health_failed`). **`deploy`** clears hash rows for removed allocations. See [deploy.md](./deploy.md#inspect-state) and [deploy-debugging.md](./deploy-debugging.md).

### `maand cat kv`

```bash
maand cat kv [--jobs j1,j2] [--active] [--deleted]
```

| Flag | Description |
|------|-------------|
| `--jobs` | Comma-separated job names; limits output to KV namespaces accessible to that job (same as job commands/templates: `maand`, `vars/bucket`, worker/allocation namespaces, upstream demand jobs) |
| `--active` | Only keys with `deleted=0` |
| `--deleted` | Only keys with `deleted=1` |

With **`--jobs`**, lists every KV namespace the job can read: shared `maand` and `vars/bucket`, each allocated worker's `maand/worker/<ip>` and tags, job/allocation namespaces, and upstream jobs referenced in command **demands**.

---

## Recommended command order

```text
maand init
# edit workspace
maand build
maand deploy              # or: maand deploy -b
maand health_check        # optional
# day-2: maand job *, maand job_command, maand run_command, maand gc
```

Operations: [disabled.md](./disabled.md), [rolling-upgrade.md](./rolling-upgrade.md), [deploy-debugging.md](./deploy-debugging.md).

Tutorials: [getting-started.md](./tutorials/getting-started.md), [day-2-operations.md](./tutorials/day-2-operations.md).
