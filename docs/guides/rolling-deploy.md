# Rolling upgrades and rolling worker reboots

Maand rolls out **job** changes through **`maand deploy`** (content hash and/or version). **Worker host** reboots and ad-hoc restarts use **`maand job`** and **`maand run_command`**. This guide covers both.

---

## Rolling job upgrade (deploy)

Deploy is the primary way to roll out workspace changes. Maand compares **content hashes** and **version targets** per allocation, rsyncs when needed, then runs **`start`**, **`restart`**, **`reload`**, or nothing — according to manifest policy and optional CLI flags.

### When an allocation is touched

| Situation | What deploy does |
|-----------|------------------|
| First deploy on worker | **`make start`** |
| Content changed since last promote | Rsync, then lifecycle per **`restart_policy`** |
| Version bumped, files unchanged | Same lifecycle (often **`reload`** when policy is `reload`) |
| Already promoted | **Skip** that job |
| **`--force`** | Roll all active allocations (policy still applies to *how*) |
| **`--sync-only`** | Rsync + promote only; **errors** if **`start`** is required |

See [deploy.md](../reference/cli/deploy.md#applying-changes-on-workers) for **`restart_policy`**, **`restart_globs`**, and **`--sync-only`**.

### Choosing a lifecycle strategy

| Job type | Typical manifest | Makefile |
|----------|------------------|----------|
| Stateful (Postgres, Kafka) | `"restart_policy": "always"` | `restart` recreates or restarts service |
| Monitoring (Prometheus) | `"restart_policy": "reload"` | `reload` calls `/-/reload` |
| App with compose + static config | `"restart_policy": "reload"` + **`restart_globs`** for compose/Dockerfile | `reload` for config; `restart` when globs match |
| Files consumed without process hook | `"restart_policy": "never"` or **`--sync-only`** | Process watches disk or you run `maand job run … --target reload` |

Add a **`reload:`** target whenever policy is **`reload`**. Without it, deploy still invokes `make reload` and the target must exist.

### Configure batch size

Two manifest fields control rollout batching:

| Field | Phase | Default | Behavior |
|-------|-------|---------|----------|
| **`max_concurrent_starts`** | First deploy (**start** new allocations) | `0` (= all at once) | Start new allocations in batches of N; **one health check** after all batches |
| **`max_concurrent_upgrades`** | Upgrades (**restart** / **reload** changed allocations) | `1` | Lifecycle in batches of N; **health check after each batch** when a target runs |

In **`workspace/jobs/<job>/manifest.json`**:

```json
{
  "version": "2.0.0",
  "selectors": ["worker"],
  "max_concurrent_starts": 2,
  "max_concurrent_upgrades": 2
}
```

Both fields use **`rollout_order`** (KV key `maand/job/<job>/rollout_order`, synced from catalog on build) to pick worker order within each batch. Override in **`pre_deploy`** with **`put_rollout_order()`** — see [job-command-api.md](../reference/job-command-api.md). Deploy validates softly and falls back to catalog order if the list is stale.

Example: job **`api`** on workers `10.0.0.1` … `10.0.0.4` with **`max_concurrent_upgrades: 2`** and **`restart_policy: always`**:

```text
Batch 1: restart 10.0.0.1 and 10.0.0.2 (parallel)
         → health_check (wait) for the job
Batch 2: restart 10.0.0.3 and 10.0.0.4 (parallel)
         → health_check again
```

With **`restart_policy: reload`**, the same batches call **`make reload`** instead (or **`restart`** on workers where **`restart_globs`** matched).

Set **`max_concurrent_upgrades`** to the largest restart burst you can tolerate while keeping the service healthy (often 1 for stateful jobs, higher for stateless). Use **`max_concurrent_starts`** the same way for **first deploy** when bringing up a multi-node cluster (Vault, Cassandra, Postgres primaries/replicas).

After each batch start, restart, or reload, maand runs **`after_allocation_started`** hooks (if registered), then the health gate for that phase.

### Version and Makefile env

On **start**, **restart**, and **reload**, the worker Makefile receives:

```text
CURRENT_VERSION=<running, pre-promote>
NEW_VERSION=<target from build>
```

Use them for migration scripts:

```makefile
restart:
	./bin/upgrade.sh "$(CURRENT_VERSION)" "$(NEW_VERSION)"
	$(MAKE) start
```

After a successful deploy wave, **`promoteAllocationHash`** sets **`current_version = new_version`** in the catalog.

### Deployment sequence (`deployment_seq`)

Jobs with **demands** deploy in waves by **`deployment_seq`** (lower first). Within one sequence value, jobs are independent; each job applies its own **`max_concurrent_starts`** (starts) and **`max_concurrent_upgrades`** (restarts).

```bash
maand cat jobs                    # deployment_seq column
maand deploy --jobs api,worker    # still respects seq order
```

See [deployment-sequence.md](../reference/deployment-sequence.md).

### `job_control` custom rollout

If any manifest command uses **`executed_on: ["job_control"]`**, default Makefile lifecycle (start/restart/reload) is **not** used. Your script receives:

```text
NEW_ALLOCATIONS=10.0.0.1,10.0.0.2
UPDATED_ALLOCATIONS=10.0.0.3
CURRENT_VERSION=...
NEW_VERSION=...
```

Implement canary or blue/green logic inside the command. See [job-command-api.md](../reference/job-command-api.md).

### Recommended upgrade flow

```bash
# 1. Change workspace (manifest version, files, templates)
vim workspace/jobs/api/manifest.json

# 2. Plan
maand build
maand deploy --dry-run

# 3. Roll out
maand deploy
# or: maand deploy --jobs api

# 4. Verify
maand cat deployments --jobs api
maand health_check --jobs api --wait
```

### Version-only upgrade (no file change)

Bump **`version`** in **`manifest.json`** only:

```bash
maand build
maand deploy --dry-run    # should show restart
maand deploy
```

### Force redeploy (same tree)

After operator-initiated reroll:

```bash
maand deploy --force --jobs api
```

---

## Rolling job restart (without deploy)

Use when the catalog is already promoted but processes need a bounce (config reload, memory leak, etc.).

### All allocations

```bash
maand job restart api
maand health_check --jobs api --wait    # optional
```

### One worker at a time

```bash
for ip in 10.0.0.1 10.0.0.2 10.0.0.3; do
  maand job restart api --allocations "$ip"
  maand health_check --jobs api --wait
done
```

### Custom Makefile target

```bash
maand job run api --target reload
```

Requires **`maand deploy`** to have run at least once (`worker.json` / **`update_seq`** in sync).

---

## Rolling worker reboot

Host OS reboot patterns (disable → reboot → re-enable): [worker-reboot.md](./worker-reboot.md).

---

## Health checks during rolling work

| Step | Command |
|------|---------|
| After each deploy batch | Automatic when job defines manifest probes or **`health_check`** commands |
| After manual restart | `maand health_check --jobs api --wait` |
| After worker reboot | `maand health_check --wait` or per-job **`--jobs`** |
| After `run_command` batch | `maand run_command ... --health_check` |

---

## Inspect rolling state

```bash
maand deploy --dry-run
maand cat deployments --jobs api
maand cat allocations --jobs api
```

During deploy, different workers may briefly show different **`current_version`** values until each batch promotes.

---

## Related

- [deploy.md](../reference/cli/deploy.md) — full deploy pipeline and version tracking
- [disable and drain](disable-and-drain.md) — drain workers or jobs
- [job.md](../reference/cli/job.md) — manual start/stop/restart/reload
- [run-command.md](../reference/cli/run-command.md) — SSH batches and **`--concurrency`**
- [health-check.md](../reference/cli/health-check.md) — probes and wait/retry
- [debugging-deploy.md](debugging-deploy.md) — when rolling upgrade stalls or fails
