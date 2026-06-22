# Rolling upgrades and rolling worker reboots

Maand rolls out **job** changes through **`maand deploy`** (content hash and/or version). **Worker host** reboots and ad-hoc restarts use **`maand job`** and **`maand run_command`**. This guide covers both.

---

## Rolling job upgrade (deploy)

### When deploy restarts allocations

Deploy restarts (or starts) an allocation when:

| Trigger | Detection |
|---------|-----------|
| First deploy on worker | `previous_hash IS NULL` → **start** |
| Workspace content changed | `previous_hash != current_hash` → **restart** |
| Version-only bump | `hash.current_version != allocations.new_version` → **restart** |
| **`--force`** | **restart** all active allocations (except new → still **start**) |

Skipped when hashes and versions are already promoted on all **active** allocations.

### Configure batch size

Two manifest fields control rollout batching:

| Field | Phase | Default | Behavior |
|-------|-------|---------|----------|
| **`deploy_parallel_count`** | First deploy (**start** new allocations) | `0` (= all at once) | Start new allocations in batches of N; **one health check** after all batches |
| **`update_parallel_count`** | Upgrades (**restart** changed allocations) | `1` | Restart in batches of N; **health check after each batch** |

In **`workspace/jobs/<job>/manifest.json`**:

```json
{
  "version": "2.0.0",
  "selectors": ["worker"],
  "deploy_parallel_count": 2,
  "update_parallel_count": 2
}
```

Both fields use **`deploy_order`** (KV key `maand/job/<job>/deploy_order`, synced from catalog on build) to pick worker order within each batch. Override in **`pre_deploy`** with **`put_deploy_order()`** — see [job-command-api.md](../reference/job-command-api.md). Deploy validates softly and falls back to catalog order if the list is stale.

Example: job **`api`** on workers `10.0.0.1` … `10.0.0.4` with **`update_parallel_count: 2`**:

```text
Batch 1: restart 10.0.0.1 and 10.0.0.2 (parallel)
         → health_check (wait) for the job
Batch 2: restart 10.0.0.3 and 10.0.0.4 (parallel)
         → health_check again
```

Set **`update_parallel_count`** to the largest restart burst you can tolerate while keeping the service healthy (often 1 for stateful jobs, higher for stateless). Use **`deploy_parallel_count`** the same way for **first deploy** when bringing up a multi-node cluster (Vault, Cassandra, Postgres primaries/replicas).

After each batch start/restart, maand runs **`after_allocation_started`** hooks (if registered), then the health gate for that phase.

### Version and Makefile env

On **start** and **restart**, the worker Makefile receives:

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

Jobs with **demands** deploy in waves by **`deployment_seq`** (lower first). Within one sequence value, jobs are independent; each job applies its own **`deploy_parallel_count`** (starts) and **`update_parallel_count`** (restarts).

```bash
maand cat jobs                    # deployment_seq column
maand deploy --jobs api,worker    # still respects seq order
```

See [deployment-sequence.md](../reference/deployment-sequence.md)).

### `job_control` custom rollout

If any manifest command uses **`executed_on: ["job_control"]`**, default start/restart is **not** used. Your script receives:

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

After **`maand health_check --update-hash`** or operator-initiated reroll:

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
- [disabled.md](disable-and-drain.md) — drain workers or jobs
- [job.md](../reference/cli/job.md) — manual start/stop/restart
- [run-command.md](../reference/cli/run-command.md) — SSH batches and **`--concurrency`**
- [health-check.md](../reference/cli/health-check.md) — probes and wait/retry
- [deploy-debugging.md](debugging-deploy.md) — when rolling upgrade stalls or fails
