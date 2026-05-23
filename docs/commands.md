# Command reference

Run `maand` from the **bucket root** (the directory created by `maand init`). Use `maand <command> --help` for flags.

## Workflow commands

| Command | Summary | Details |
|---------|---------|---------|
| `maand init` | Create or upgrade bucket (DB, workspace layout, CA, secrets) | — |
| `maand build` | Read workspace → update `maand.db`, KV, certs; run `post_build` hooks | [build.md](./build.md) |
| `maand deploy` | Push jobs to workers, roll out, run deploy hooks | [deploy.md](./deploy.md) |
| `maand health_check` | Run `health_check` job commands on active allocations | [health-check.md](./health-check.md) |
| `maand gc` | Purge removed allocations, worker data, old KV history | [gc.md](./gc.md) |

## Inspect commands

| Command | Summary |
|---------|---------|
| `maand info` | Bucket ID, update sequence, counts | [info.md](./info.md) |
| `maand cat workers` | Worker catalog |
| `maand cat jobs` | Job catalog (includes **`deployment_seq`**) |
| `maand cat allocations` | Job × worker rows (`--jobs`, `--workers` filters) |
| `maand cat job_commands` | Commands from manifests |
| `maand cat job_ports` | Declared ports per job |
| `maand cat kv` | List KV keys (or `maand cat kv get <ns> <key>`) |

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

- `data/maand.db` with schema
- `workspace/workers.json` (empty `[]`), `workspace/jobs/`
- `maand.conf` defaults (`ssh_user`, `ssh_key`, `use_sudo`, cert TTL)
- Bucket CA in `secrets/ca.crt` / `ca.key`
- KV encryption key

Does not contact workers.

---

## `maand build`

```bash
maand build
```

Reconciles the entire workspace. No filters. Validates job **demands** and **version** constraints. See [build.md](./build.md#job-version).

**Host prerequisites:** `python3`, `bun` (if TS/JS commands), for `post_build` hooks.

---

## `maand deploy`

```bash
maand deploy [--build] [--jobs j1,j2] [--dry-run]
```

| Flag | Description |
|------|-------------|
| `-b`, `--build` | Run `maand build` first |
| `--jobs` | Limit to named jobs (still respects deployment sequence) |
| `-n`, `--dry-run` | Compare allocation hashes; no worker changes |

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
maand health_check [--jobs j1,j2] [--wait] [--verbose]
```

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
maand cat job_commands
maand cat job_ports
maand cat kv
maand cat kv get maand/job/api name
```

See [info.md](./info.md).

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

Tutorials: [getting-started.md](./tutorials/getting-started.md), [day-2-operations.md](./tutorials/day-2-operations.md).
