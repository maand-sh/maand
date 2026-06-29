# 11. Health and monitoring

---

## Health checks

**`maand health_check`** verifies workers and jobs after deploy or on demand:

1. SSH gate (worker reachable)
2. Manifest **probes** (tcp/http/ssh) if declared
3. **`health_check`** job command if probes pass or aren't defined

```bash
maand health_check --jobs api
maand health_check --jobs api --wait --verbose
```

Deploy can run health checks between rolling batches when configured.

Many jobs also define a **`test`** Makefile target for application-level checks (HTTP curl, etc.) — deploy does not call it automatically unless you wire that in a hook.

Reference: [health-check.md](../reference/cli/health-check.md).

---

## Prometheus (optional)

Jobs may include **`_prometheus/`**:

- Scrape config snippets
- Alert rules, runbooks, dashboards

Build validates and stores in `job_files`; scrape configs sync to KV for a prometheus job to consume.

Guide: [prometheus.md](../guides/prometheus.md).

---

## Observability (where to read more)

| Concern | Document |
|---------|----------|
| Deploy/rsync debug output | [logging.md](../reference/observability/logging.md) — `maand logs show`, `logs/<worker>.log` |
| Deploy skipped or partial | [debugging-deploy.md](../guides/debugging-deploy.md) |
| App output on workers | Your Makefile (compose, systemd) or SSH — same as without maand |

Maand records **its own** command history on the CLI host. Application stdout and metrics stay in whatever stack you run on workers.

---

## Next

[12 — Day-2 operations](./12-day-2-operations.md).
