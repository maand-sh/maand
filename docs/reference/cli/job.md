# `maand job`

Manual **job control** on workers: start, stop, restart, reload, custom Makefile targets (for example `migrate`), or show status. Validates that each worker’s `worker.json` matches maand.db before running.

**`reload`** is a first-class Makefile target — use **`maand job run <job> --target reload`** for a manual soft apply. During **`maand deploy`**, lifecycle is driven by manifest **`restart_policy`** (**`always`** → restart, **`reload`** → reload, **`never`** → none); deploy does not call **`maand job`**.

Requires a prior **`maand deploy`** so workers are in sync (`update_seq` check).

For job definitions, manifest fields, command **demands**, and **`deployment_seq`** (deploy wave order), see [manifest.md](../manifest.md) and [deployment-sequence.md](../deployment-sequence.md).

## CLI

```bash
maand job start <job> [--allocations ip,...] [--health_check]
maand job stop <job> [--allocations ip,...] [--health_check]
maand job restart <job> [--allocations ip,...] [--health_check]
maand job run <job> --target <makefile-target> [--allocations ip,...] [--health_check]
maand job status <job> [--allocations ip,...]
maand job create <job> [--selectors s1,s2]
```

| Flag | Commands | Description |
|------|----------|-------------|
| `--allocations` | control | Comma-separated worker IPs (default: all workers for the job). |
| `--target` | `run` | Makefile target (required): built-in `start`, `stop`, `restart`, `reload`, `status`, or any safe custom name such as `migrate`. |
| `--health_check` | start/stop/restart/run | Run health_check commands after the action. |
| `--selectors` | `create` | Selectors written into a new `manifest.json`. |

## Worker sync check

Before control commands run, maand verifies each worker’s `/opt/worker/<bucket_id>/worker.json` matches **`bucket_id`**, **`worker_id`**, and **`update_seq`** in the database. On mismatch you get an error — run **`maand deploy`** first.

## vs deploy

| | `maand deploy` | `maand job *` |
|--|----------------|---------------|
| Purpose | Roll out changed jobs, hooks, hashes | Manual lifecycle / Makefile targets |
| Sync check | No | Yes (`worker.json` / `update_seq`) |
| Hash / skip logic | Yes | No |

**`maand job`** always runs **`runner.py`** → Makefile targets on workers. The **`job_control`** command event replaces Makefile lifecycle during **`maand deploy`** only — not during manual `maand job` commands.

## Examples

```bash
maand job restart api
maand job run api --target reload
maand job stop api --allocations 10.0.0.2
maand job run api --target migrate
maand job create myservice --selectors worker
```
