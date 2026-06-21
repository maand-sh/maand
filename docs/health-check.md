# `maand health_check`

**Health check** verifies workers are reachable, then checks each job on active allocations.

Each job may use **manifest probes**, a **custom command**, or **both**:

- **Manifest probes** — `health_check.checks` in `manifest.json` (tcp / http / ssh)
- **Custom command** — `command_*` with `executed_on: ["health_check"]`

When both are defined, manifest probes run first, then `health_check` commands (in DB order).

Order when you run **`maand health_check`**:

1. **Worker health** — TCP dial to each worker’s **SSH port** (`maand.conf` `ssh_port`, default **22**).
2. **Job health** — manifest probes (if any), then `health_check` command scripts (if any).

Deploy runs **job** health automatically after **restart** / **job_control** for jobs that define one of the above. Deploy does **not** re-run the worker SSH gate on every job (use `maand health_check` for that).

---

## CLI

```bash
maand health_check [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--jobs` | all jobs | Comma-separated job names. Unknown names error. |
| `--wait` | false | Retry until success or **30 attempts** (1 second apart). |
| `--verbose` | false | Stream command output. |
| `--update-hash` | false | Mark failed allocations for redeploy when **`health_check` commands** fail (manifest probes only; no effect). |

Examples:

```bash
maand health_check
maand health_check --jobs api,worker
maand health_check --jobs api --wait --verbose
maand health_check --update-hash --jobs vault
```

---

## Prerequisites

1. **`maand build`** (allocations and `job_commands` in DB).
2. **Host tools**: `python3`, `bun` (if needed), `bash`, `ssh`.
3. Jobs need a **`health_check`** section in `manifest.json` and/or a command with **`executed_on`: `["health_check"]`**.

If a job has **neither**:

```text
health check skipped: <job> (no health_check config or commands)
```

That is **not** an error; exit code remains 0 for that job.

---

## Built-in manifest health (recommended)

Declare probes next to `resources.ports`. Port names reference manifest keys; maand resolves assigned numbers at check time and probes **each active allocation** (`worker_ip:port`).

```json
{
  "selectors": ["cassandra"],
  "resources": {
    "ports": {
      "cassandra_cql_port": {},
      "cassandra_http_port": {}
    }
  },
  "health_check": {
    "checks": [
      { "type": "tcp", "port": "cassandra_cql_port" },
      { "type": "http", "port": "cassandra_http_port", "path": "/metrics", "expect_status": 200 }
    ],
    "timeout_seconds": 5,
    "wait": { "attempts": 30, "interval_seconds": 1 }
  }
}
```

| Probe | Fields |
|-------|--------|
| **`tcp`** | `port` (required) |
| **`http`** | `port`, `path` (default `/`), `expect_status` (default `200`), `scheme` (default `http`) |
| **`ssh`** | `command` (required) — one shell line on the **worker** over SSH (no job workspace staging) |

All checks must pass on **every** allocation (AND). Built-in probes need no Python/Bun script.

Example **`ssh`** probe (systemd on the worker):

```json
{ "type": "ssh", "command": "systemctl is-active cassandra" }
```

---

## Custom command health (escape hatch)

```json
{
  "selectors": ["worker"],
  "commands": {
    "command_health": {
      "executed_on": ["health_check"]
    }
  }
}
```

Script: `_modules/command_health.py`. Use for cluster readiness (`nodetool status`, etc.) when manifest probes are not enough. You may combine with manifest probes; commands run after probes pass.

**Health-fast workspace:** for `health_check` commands only, maand stages **`_modules/command_<name>.*`**, embedded `maand.py` / `maand.ts`, and certs — not the full job tree (Makefile, templates, etc.).

---

## Worker SSH health

Before job checks, `maand health_check` dials **`worker_ip:ssh_port`** for every worker in `workers.json`.

Configure in `maand.conf`:

```toml
ssh_port = 22
```

Output:

```text
worker health check passed
```

or retry/fail when `--wait` is set.

---

## What happens internally

```text
Open maand.db
Begin transaction
kv.Initialize + StartRuntimeAPI (localhost:8080 for scripts)
CheckWorkers (SSH TCP on all workers)
Resolve job list (--jobs filter or all jobs from DB)
Run jobs in parallel (up to 4 jobs at a time)
  For each job:
    Manifest health_check probes (tcp/http/ssh per allocation), if defined
    Then each health_check command (in DB order), if defined:
      jobcommand.JobCommand(..., event="health_check", concurrency=1)
Commit transaction on success (or on failure when --update-hash marked allocations)
```

### Per-allocation execution

**Built-in probes** dial `worker_ip:assigned_port` from the CLI host (no scripts).

**Custom commands** — for each **active** worker hosting the job:

1. Stage job files under `tmp/workers/<ip>/jobs/<job>/` (from `job_files` + embedded `maand.py` / `maand.ts` + certs from KV).
2. Run script on the CLI host with env:
   - `ALLOCATION_ID`, `ALLOCATION_IP`, `JOB`, `EVENT=health_check`, `COMMAND=<name>`, `DISABLED`
   - `JOB_COMMAND_API_HOST` → `127.0.0.1`

Failures on one worker fail the job; multiple jobs can fail in one run (**batch error**).

---

## `--wait` behavior

When **`--wait`** is set (deploy uses wait mode internally after restarts):

- Run built-in probes and `health_check` commands for the job.
- On failure, sleep **1 second** and retry.
- Up to **30 attempts** per job.
- Success prints: `health check passed: <job>`
- Failure returns **`HealthCheckError`** with the last underlying error.

Without `--wait`, a single failure prints `health check failed: <job>` and returns immediately.

---

## `--update-hash` and redeploy

When **`--update-hash`** is set and a **`health_check` command** exits non-zero on a worker, that allocation is marked for redeploy (`previous_hash` diverges from `current_hash`). Marks are **committed even when the health check fails**.

Does **not** apply to manifest probe failures — use **`maand deploy --force`** to roll those jobs again.

```bash
maand health_check --update-hash --jobs vault   # command-based jobs
maand deploy --jobs vault                       # restarts marked allocations only

maand deploy --force --jobs cassandra           # force full redeploy (e.g. after manifest probe issues)
```

---

## Relationship to deploy

| Context | `wait` | `verbose` | `--update-hash` |
|---------|--------|-----------|-----------------|
| `maand health_check` | User-controlled (`--wait`) | User-controlled (`--verbose`) | CLI only |
| Deploy after **restart** / **job_control** | **true** (wait for recovery) | **true** | not used |

Production deploy waits for health to pass after rolling updates; ad-hoc CLI checks can be one-shot unless you pass `--wait`. See [`deploy.md`](./deploy.md#which-jobs-run-in-a-deploy-wave) for **`--force`**.

---

## Relationship to build / demands

- **`health_check`** is a valid `executed_on` value at build time.
- It does **not** affect **`deployment_seq`** unless paired with demands (demands are between jobs; `health_check` alone does not create edges).
- **`post_build`** hooks run at end of build; **`health_check`** does not run automatically on build.

---

## Parallelism

- **Between jobs**: up to **4** concurrent jobs (`defaultJobParallelism`).
- **Within a job**: **concurrency 1** (commands and allocations run sequentially for health_check in `JobCommand` when called with concurrency 1).

---

## Errors

| Error | Meaning |
|-------|---------|
| `HealthCheckError` | One job failed (wraps command/SSH/script error). |
| Batch error | Multiple jobs failed; message lists each job. |
| `jobs not in this bucket: [...]` | Bad `--jobs` name. |
| `NotFoundError` | Command not registered for `health_check` on that job. |

---

## Typical usage patterns

**After deploy:**

```bash
maand deploy -b
maand health_check --wait
```

**Single job smoke test:**

```bash
maand health_check --jobs myservice --verbose
```

**CI gate:**

```bash
maand health_check --wait && echo OK
```

**Mark unhealthy nodes for redeploy** (command-based health only):

```bash
maand health_check --update-hash --jobs vault
maand deploy --jobs vault
```

**Force redeploy** without workspace or hash changes:

```bash
maand deploy --force --jobs api
```

---

## Writing health check scripts

Use the same libraries as other events — see [`job-command.md`](./job-command.md):

- Python: `maand.py` → KV get/put, demands, semaphores.
- Bun: `maand.ts` → same API.

Scripts should exit **0** when healthy, non-zero when not. Keep checks read-only when possible; mutations go to KV and persist only if you also run deploy or call persist APIs appropriately.

---

## Related commands

- [`deploy.md`](./deploy.md) — automatic health check after rollout.
- [`job-command.md`](./job-command.md) — events, runtime API, demands.
- [`build.md`](./build.md) — register commands in the catalog.
