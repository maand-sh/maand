# `maand run_command`

**run_command** runs an arbitrary shell command on one or more workers over SSH from the maand CLI host.

## CLI

```bash
maand run_command [command] [flags]
```

| Flag | Description |
|------|-------------|
| `--workers` | `-w` | Comma-separated worker IPs (default: all workers). |
| `--labels` | `-l` | Comma-separated worker labels. |
| `--concurrency` | `-c` | Workers per batch (parallel SSH sessions). |
| `--health_check` | | Run health checks for all jobs after every batch. |

Examples:

```bash
maand run_command "uptime"
maand run_command "df -h /opt/worker" --workers 10.0.0.1,10.0.0.2
maand run_command "hostname" --concurrency 4
```

## Prerequisites

- Initialized bucket with workers in `workspace/workers.json` and a prior **`maand build`**.
- SSH key at `secrets/<ssh_key>` (from `maand.conf`) authorized on workers.
- Host tools: `bash`, `ssh` (checked before SSH).
- Target workers must have `bash` and `timeout` on `PATH` (`sudo` when `use_sudo = true`).

## Notes

- Commands run on workers as the configured **`ssh_user`** (default `agent`).
- **Working directory:** scripts start in the worker's bucket directory **`/opt/worker/$BUCKET_ID`**, so relative paths like `jobs/<job>/...` resolve directly. If the bucket has not been deployed yet (no such directory), it falls back to the SSH login directory so host-bootstrap commands still work.
- **`BUCKET_ID`** and **`WORKER_IP`** are exported into every script.
- This is separate from **job commands** (`pre_deploy`, `job_control`, etc.) which use the runtime API and job workspaces.
