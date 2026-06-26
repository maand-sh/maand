# Tutorial: Day-2 operations

After [getting started](../start/quickstart.md), use these patterns for everyday cluster operations. Assumes a working bucket with deployed jobs.

---

## Inspect catalog state

Quick summary:

```bash
maand info
```

Detailed tables:

```bash
maand cat workers
maand cat jobs
maand cat allocations
maand cat job_commands
maand cat job_ports
maand cat certs
maand cat kv
```

Filter allocations:

```bash
maand cat allocations --jobs api,worker
maand cat allocations --workers 10.0.0.1
```

Check TLS expiry (CA + job leaf certs):

```bash
maand cat certs
maand cat certs --jobs api,postgres
```

Read one KV key:

```bash
maand cat kv get maand/job/api name
```

---

## Manual job control

`maand job` runs Makefile targets (or **`job_control`** commands) on workers. It verifies each worker’s `worker.json` matches the database — run **`maand deploy`** first if you see sync errors.

```bash
maand job restart api
maand job stop api --allocations 10.0.0.2
maand job start api --health_check
maand job run api --target migrate
maand job status api
```

| vs deploy | `maand deploy` | `maand job` |
|-----------|----------------|-------------|
| When | Catalog or job files changed | Ops / one-off lifecycle |
| Sync check | No | Yes (`update_seq`) |
| Hash skip | Yes | No |

See [job.md](../reference/cli/job.md).

---

## Health checks

Each job may use **manifest probes**, a **custom command**, or **both** (probes run first):

**Option A — manifest probes** (tcp/http/ssh) in `manifest.json` — see [health-check.md](../reference/cli/health-check.md#built-in-manifest-health-recommended).

**Option B — custom command:**

```json
"commands": {
  "command_health": {
    "executed_on": ["health_check"]
  }
}
```

Add **`workspace/jobs/api/_modules/command_health.py`** (see [job-command-api.md](../reference/job-command-api.md)).

Run checks:

```bash
maand health_check
maand health_check --jobs api --wait --verbose
```

Mark unhealthy command-based allocations and redeploy:

```bash
maand health_check --update-hash --jobs api
maand deploy --jobs api
```

Force a full redeploy without workspace changes:

```bash
maand deploy --force --jobs api
```

Deploy runs health checks automatically after restart when health is configured.

See [health-check.md](../reference/cli/health-check.md).

---

## Prometheus monitoring

Add **`_prometheus/`** under each job that exposes metrics (see [prometheus.md](prometheus.md)):

```text
workspace/jobs/api/_prometheus/
├── scrape.yaml
├── alerts/
└── runbooks/
```

After adding or changing scrape configs:

```bash
maand build
maand deploy --jobs api,...      # app jobs first
maand deploy --jobs prometheus   # refresh scrape config, alert rules, runbook HTML
```

Runbooks are served from the Prometheus UI after deploy (`/consoles/runbooks/...`) — see [prometheus.md](prometheus.md#runbooks).

---

## Ad-hoc commands on workers

**`maand run_command`** runs shell on workers (not job workspaces):

```bash
maand run_command "uptime"
maand run_command "df -h /opt/worker" --workers 10.0.0.1,10.0.0.2
maand run_command "hostname" --labels worker --concurrency 4
maand run_command "systemctl status myservice" --health_check
```

Host needs `bash` and `ssh`; workers need `bash` and `timeout`.

See [run-command.md](../reference/cli/run-command.md).

---

## Disable an allocation temporarily

See [disabled.md](disable-and-drain.md) for the full guide (per-allocation, per-job, per-worker, re-enable).

Create or edit **`workspace/disabled.json`**:

```json
{
  "jobs": {
    "api": {
      "allocations": ["10.0.0.2"]
    }
  }
}
```

Disable every job on a worker:

```json
{
  "workers": ["10.0.0.3"]
}
```

Disable an entire job everywhere:

```json
{
  "jobs": {
    "api": {}
  }
}
```

Then:

```bash
maand build
maand deploy
```

Disabled allocations are skipped for start/restart/rsync; deploy **stops** them if running and **keeps** artifacts and KV. Re-enable: clear **`disabled.json`**, **`maand build`**, **`maand deploy`**.

---

## Remove a worker or job

1. Remove the host from **`workers.json`** or delete **`workspace/jobs/<name>/`**
2. **`maand build`** — marks related allocations **`removed = 1`**
3. **`maand deploy`** — stops jobs, removes deployed job files (keeps `data/` and `logs/` on workers)
4. **`maand gc`** — deletes worker `data/`/`logs/`/`bin/`, allocation rows, and KV references

```bash
# after editing workspace
maand build
maand deploy
maand gc
maand gc --retain-days 7   # keep deleted KV history longer
```

See [gc.md](../reference/cli/gc.md).

---

## Partial deploy and dry-run

Check whether deploy would change anything:

```bash
maand deploy --dry-run
```

Deploy only specific jobs (still ordered by `deployment_seq`):

```bash
maand deploy --jobs api,worker
```

If deploy fails partway, fix the issue and re-run — hash tracking resumes unchanged allocations.

Force redeploy when content is already promoted:

```bash
maand deploy --force --jobs api
maand deploy --dry-run --force    # preview
```

See [deploy.md](../reference/cli/deploy.md).

---

## Per-job config overrides

Optional **`workspace/bucket.jobs.conf`**:

```toml
[api]
memory = "512 mb"
```

If **`maand.conf`** sets `job_config_selector = "prod"`, use **`bucket.jobs.prod.conf`** instead.

After editing:

```bash
maand build
maand deploy -b
```

---

## Upgrade maand schema

When upgrading the maand binary:

```bash
maand init    # applies DB migrations, keeps bucket_id and CA
maand build
maand deploy
```

---

## Rolling upgrades

See [rolling-deploy](rolling-deploy.md) for **`update_parallel_count`**, version-only deploys, and rolling worker reboots.

---

## Troubleshooting checklist

See [deploy-debugging.md](debugging-deploy.md) for a full deploy troubleshooting guide.

| Symptom | Likely fix |
|---------|------------|
| `worker.json` / `update_seq` mismatch | `maand deploy` |
| Host prerequisite error | Install `ssh`/`rsync`/`python3`/`bun` on CLI host |
| Worker prerequisite error | Install `make`/`python3`/`rsync` on worker; fix `sudo` |
| No allocations for job | Check selectors vs worker labels; run `maand build` |
| Build resource error | Add `memory`/`cpu` to workers or lower job limits |
| Port collision | Remove duplicate port names; maand assigns unique numbers from the pool |
| `ErrPortRangeExhausted` | Widen `port_min`/`port_max` in `bucket.conf` or remove unused jobs/ports |
| `ErrInvalidJobVersion` | Add or fix `version` on jobs in the dependency graph |
| `ErrJobCommandDemandVersionMismatch` | Bump upstream job version or relax `min_version`/`max_version` |
| Upgrade script needs old/new release | Read `CURRENT_VERSION` / `NEW_VERSION` in Makefile or job command env — [deploy.md](../reference/cli/deploy.md#allocation-version-tracking) |

Concept reference: [concepts.md](../start/concepts.md)
