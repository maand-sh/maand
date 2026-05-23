# `maand health_check`

**Health check** runs all job commands registered for the **`health_check`** event against **active allocations** of each selected job. It uses the same **jobcommand** machinery as deploy (CLI host, staged workspaces, Python/Bun scripts).

Deploy also runs health checks automatically after **restart** / **job_control** when commands exist.

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

Examples:

```bash
maand health_check
maand health_check --jobs api,worker
maand health_check --jobs api --wait --verbose
```

---

## Prerequisites

1. **`maand build`** (allocations and `job_commands` in DB).
2. **Host tools**: `python3`, `bun` (if needed), `bash`, `ssh`.
3. Jobs must define at least one command with **`executed_on`** including **`health_check`** in `manifest.json`.
4. Command scripts present: `workspace/jobs/<job>/_modules/command_<name>.py` (or `.ts` / `.js`).

If a job has **no** `health_check` commands:

```text
health check skipped: <job> (no health_check commands)
```

That is **not** an error; exit code remains 0 for that job.

---

## Manifest example

```json
{
  "selectors": ["worker"],
  "commands": {
    "command_health": {
      "executed_on": ["health_check"],
      "demands": { "job": "", "command": "", "config": {} }
    }
  }
}
```

Command name must start with **`command_`**. Script file: `_modules/command_health.py`.

---

## What happens internally

```text
Open maand.db
Begin transaction
kv.Initialize + StartRuntimeAPI (localhost:8080 for scripts)
Resolve job list (--jobs filter or all jobs from DB)
Run jobs in parallel (up to 4 jobs at a time)
  For each job:
    For each health_check command name (in DB order):
      jobcommand.JobCommand(..., event="health_check", concurrency=1)
Commit transaction on success
```

### Per-allocation execution

For each **active** worker hosting the job:

1. Stage job files under `tmp/workers/<ip>/jobs/<job>/` (from `job_files` + embedded `maand.py` / `maand.ts` + certs from KV).
2. Run script on the CLI host with env:
   - `ALLOCATION_ID`, `ALLOCATION_IP`, `JOB`, `EVENT=health_check`, `COMMAND=<name>`, `DISABLED`
   - `JOB_COMMAND_API_HOST` → `127.0.0.1`

Failures on one worker fail the job; multiple jobs can fail in one run (**batch error**).

---

## `--wait` behavior

When **`--wait`** is set (deploy uses wait mode internally after restarts):

- Run all `health_check` commands for the job.
- On failure, sleep **1 second** and retry.
- Up to **30 attempts** per job.
- Success prints: `health check passed: <job>`
- Failure returns **`HealthCheckError`** with the last underlying error.

Without `--wait`, a single failure prints `health check failed: <job>` and returns immediately.

---

## Relationship to deploy

| Context | `wait` | `verbose` |
|---------|--------|-----------|
| `maand health_check` | User-controlled (`--wait`) | User-controlled (`--verbose`) |
| Deploy after **restart** / **job_control** | **true** (wait for recovery) | **true** |

So production deploy waits for health to pass after rolling updates; ad-hoc CLI checks can be one-shot unless you pass `--wait`.

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
