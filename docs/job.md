# `maand job`

Manual **job control** on workers: start, stop, restart, custom Makefile targets (for example `migrate`), or show status. Validates that each worker’s `worker.json` matches maand.db before running.

Requires a prior **`maand deploy`** so workers are in sync (`update_seq` check).

For job definitions, manifest fields, command **demands**, and **`deployment_seq`** (deploy wave order), see [jobs-and-dependencies.md](./jobs-and-dependencies.md).

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
| `--target` | `run` | Makefile target (required): built-in `start`, `stop`, `restart`, `status`, or any safe custom name such as `migrate`. |
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

Jobs with **`job_control`** commands in the manifest use those instead of the default Makefile runner when present.

## Examples

```bash
maand job restart api
maand job stop api --allocations 10.0.0.2
maand job run api --target migrate
maand job create myservice --selectors worker
```
