# 11. Health, monitoring, and logs

Three different log and health layers — don't mix them up.

---

## 1. Maand command logs (CLI host)

Maand records its own operations under **`logs/`**:

| File | Contents |
|------|----------|
| `logs/<worker_ip>.log` | Deploy, rsync, SSH, job commands for that worker |
| `logs/maand.log` | Bucket-local events |
| `logs/runs/<run_id>/` | One CLI invocation |

```bash
maand logs show --job api --worker 10.0.0.1 --format human
maand logs show --event deploy_skip --format human
```

These are **structured** lines (deploy phases, exit codes) — not your application's stdout.

Reference: [logging.md](../reference/observability/logging.md).

---

## 2. Application logs (workers)

Runtime output from your service lives on the worker:

```text
/opt/worker/<bucket_id>/jobs/<job>/logs/     # if your Makefile/app writes here
docker compose logs                          # container stdout
journalctl -u <unit>                         # systemd
```

**Snapshot** (recommended pattern): add a Makefile **`logs`** target, fetch with:

```bash
maand run_command "make -s -C jobs/api logs" --workers 10.0.0.1
```

**Live follow:** `run_command` with `docker compose logs -f` or SSH directly — see [05-jobs-and-lifecycle.md](./05-jobs-and-lifecycle.md).

---

## 3. Health checks

**`maand health_check`**:

1. SSH gate (worker reachable)
2. Manifest **probes** (tcp/http/ssh) if declared
3. **`health_check`** job command if probes pass or aren't defined

```bash
maand health_check --jobs api
maand health_check --jobs api --wait --verbose
```

Deploy can run health checks between rolling batches when configured.

Reference: [health-check.md](../reference/cli/health-check.md).

---

## Prometheus (optional)

Jobs may include **`_prometheus/`**:

- Scrape config snippets
- Alert rules, runbooks, dashboards

Build validates and stores in `job_files`; scrape configs sync to KV for a prometheus job to consume.

Guide: [prometheus.md](../guides/prometheus.md).

---

## Summary table

| What you want | Where to look |
|---------------|---------------|
| Why deploy skipped an allocation | `maand logs show`, `maand cat deployments` |
| App error lines | `make logs` + `run_command`, or SSH + `docker logs` / `journalctl` |
| Is the service up? | `maand health_check`, job `make test` target |
| Metrics/alerts | `_prometheus/` + prometheus job |

---

## Next

[12 — Day-2 operations](./12-day-2-operations.md).
