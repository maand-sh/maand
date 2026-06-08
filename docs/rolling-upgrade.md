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

### Configure batch size: `update_parallel_count`

In **`workspace/jobs/<job>/manifest.json`**:

```json
{
  "version": "2.0.0",
  "selectors": ["worker"],
  "update_parallel_count": 2
}
```

| Value | Behavior |
|-------|----------|
| `1` (default) | One worker at a time per job during restart |
| `N` | Up to **N** workers restarted **in parallel** within each batch; batches run sequentially |

Example: job **`api`** on workers `10.0.0.1` … `10.0.0.4` with **`update_parallel_count: 2`**:

```text
Batch 1: restart 10.0.0.1 and 10.0.0.2 (parallel)
         → health_check (wait) for the job
Batch 2: restart 10.0.0.3 and 10.0.0.4 (parallel)
         → health_check again
```

Set **`update_parallel_count`** to the largest burst you can tolerate while keeping the service healthy (often 1 for stateful jobs, higher for stateless).

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

Jobs with **demands** deploy in waves by **`deployment_seq`** (lower first). Within one sequence value, jobs are independent; each job applies its own **`update_parallel_count`**.

```bash
maand cat jobs                    # deployment_seq column
maand deploy --jobs api,worker    # still respects seq order
```

See [jobs-and-dependencies.md](./jobs-and-dependencies.md).

### `job_control` custom rollout

If any manifest command uses **`executed_on: ["job_control"]`**, default start/restart is **not** used. Your script receives:

```text
NEW_ALLOCATIONS=10.0.0.1,10.0.0.2
UPDATED_ALLOCATIONS=10.0.0.3
CURRENT_VERSION=...
NEW_VERSION=...
```

Implement canary or blue/green logic inside the command. See [job-command.md](./job-command.md).

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
maand cat hashes --jobs api
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

## Rolling worker reboot (host OS)

Maand has no built-in **`reboot`** command. Use **disable → stop → reboot → re-enable**.

### Pattern A — disable worker, reboot, re-enable

```bash
# 1. Drain all jobs on the host
cat > workspace/disabled.json <<'EOF'
{ "workers": ["10.0.0.3"] }
EOF
maand build && maand deploy

# 2. Reboot (SSH from maand host)
maand run_command "sudo reboot" --workers 10.0.0.3

# 3. Wait for SSH (manual or scripted sleep)

# 4. Re-enable
# Remove 10.0.0.3 from disabled.json
maand build && maand deploy
```

Disabled allocations **keep** deploy artifacts and KV; after reboot, **`deploy`** **starts** jobs again on re-enable.

### Pattern B — rolling reboot across a fleet

```bash
WORKERS="10.0.0.1 10.0.0.2 10.0.0.3"

for ip in $WORKERS; do
  echo "=== drain $ip ==="
  # Edit disabled.json to add only this worker under "workers"
  maand build && maand deploy

  maand run_command "sudo reboot" --workers "$ip"
  sleep 120   # tune for your boot time

  # Remove worker from disabled.json
  maand build && maand deploy
  maand health_check --wait
done
```

Automate **`disabled.json`** edits with a small script or config management tool.

### Pattern C — parallel SSH only (no catalog drain)

For workers where a hard reboot without drain is acceptable:

```bash
maand run_command "sudo reboot" --workers 10.0.0.1,10.0.0.2 --concurrency 1
```

**`--concurrency 1`** reboots one host at a time. Increase only if you accept multiple hosts down together.

After reboot, processes may be down until **`maand job start`** or **`maand deploy`**. Prefer Pattern A for production.

### After reboot: sync check

```bash
maand job status api --allocations 10.0.0.3
```

If **`worker.json` / `update_seq` mismatch**, run **`maand deploy`** before **`maand job`**.

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
maand cat hashes --jobs api
maand cat allocations --jobs api
```

During deploy, different workers may briefly show different **`current_version`** values until each batch promotes.

---

## Related

- [deploy.md](./deploy.md) — full deploy pipeline and version tracking
- [disabled.md](./disabled.md) — drain workers or jobs
- [job.md](./job.md) — manual start/stop/restart
- [run-command.md](./run-command.md) — SSH batches and **`--concurrency`**
- [health-check.md](./health-check.md) — probes and wait/retry
- [deploy-debugging.md](./deploy-debugging.md) — when rolling upgrade stalls or fails
