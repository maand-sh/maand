# Integration test assets

Local-only integration tests (`go test -tags=integration ./tests/integration/...`) read SSH and worker config from this directory.

**These files are not committed.** Copy the examples and add your key:

```bash
cp assets/workers.json.example assets/workers.json
cp assets/maand.conf.example assets/maand.conf
cp /path/to/your/worker.key assets/worker.key
chmod 600 assets/worker.key
```

## Required files

| File | Purpose |
|------|---------|
| `workers.json` | Worker hosts (same format as `workspace/workers.json`) |
| `worker.key` | Private SSH key authorized on workers |
| `maand.conf` | `ssh_user`, `ssh_key`, optional `use_sudo` |

## Optional

| Variable | Purpose |
|----------|---------|
| `MAAND_INTEGRATION_ASSETS` | Override assets directory (default: `<repo>/assets`) |

## Run integration tests

From the repository root (requires `CGO_ENABLED=1`, reachable workers, `bash`/`ssh`/`rsync`/`python3` on the CLI host):

```bash
go test -tags=integration ./tests/integration/... -v -timeout 15m
```

Integration tests are **not** run in GitHub CI.

## Coverage

Integration tests exercise the full local workflow against real workers: init/build/deploy (including dry-run, job filter, and rolling upgrade on content change), job control, health checks, job commands (CLI, KV, secrets), deploy hooks, run_command (worker/label filters, `--health_check`), GC, and `info`/`cat`.

Requires **python3** on the CLI host for job-command tests.

## Example `workers.json`

```json
[
  {"host": "192.168.1.161"}
]
```

Workers automatically receive the `worker` label; integration jobs use `"selectors": ["worker"]`.
