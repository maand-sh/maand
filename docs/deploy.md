# `maand deploy`

**Deploy** pushes job artifacts from the database to worker nodes, runs lifecycle actions (start / restart / stop), executes hook commands (`pre_deploy`, `post_deploy`, `job_control`), and updates **allocation hashes** so later deploys can skip unchanged jobs or resume after partial failure.

Requires a prior **`maand build`** (or `maand deploy --build`).

---

## CLI

```bash
maand deploy [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--jobs` | | Comma-separated job names. Default: all jobs (per deployment sequence). |
| `--build` | `-b` | Run `maand build` before deploy. |
| `--dry-run` | `-n` | Stage locally and compare allocation hashes; report whether deploy is required without changing workers or persisting hash updates. |

Examples:

```bash
maand deploy
maand deploy -b
maand deploy --jobs api,worker
maand deploy --dry-run
maand deploy -b -n
```

---

## Prerequisites

1. Initialized bucket with **`maand build`** completed successfully.
2. **Host tools**: `bash`, `ssh`, `rsync`, and `python3` on the CLI host (`bun` when any job command uses `.ts`/`.js`). Deploy checks these before syncing.
3. **`maand.conf`**: `ssh_user`, `ssh_key` (under `secrets/`), optional `use_sudo`.
4. Workers reachable by SSH from the maand CLI host; ensure `secrets/worker.key` is authorized on workers.
5. Each worker has **`python3`**, **`make`**, **`rsync`**, **`bash`**, and **`timeout`** on `PATH`. When `use_sudo = true`, **`sudo`** must work and **`sudo rsync --version`** must succeed. Deploy SSH-checks workers before syncing.
6. Each active job has a **`Makefile`** unless the job uses only **`job_control`** commands.

---

## High-level pipeline

```text
UpdateSeq (+1, committed)
Open DB transaction + kv.Initialize
Start job-command HTTP API on host
Stop removed/disabled allocations; remove job trees from workers (full bucket tree when the worker left workers.json)
For deployment_seq = 0 .. max:
  Prepare worker.json / jobs.json / bin/ for all workers
  Refresh plan hashes for jobs in this sequence
  For each job needing rollout:
    pre_deploy commands → KV checkpoint (into deploy transaction)
  For each job to stage:
    prepare job files + transpile .tpl + certs → tmp/workers/<ip>/jobs/<job>/
    rsync (filtered per job) to /opt/worker/<bucket_id>/
    update allocation content hashes (MD5 of staged tree)
    deployJob (start/restart OR job_control + health_check + post_deploy)
    KV checkpoint again
Final rsync: per successfully deployed job only (filtered + jobs.json refresh)
Commit transaction (even if some jobs failed — partial deploy)
Return joined errors if any job failed
```

---

## Deployment sequence

Jobs with the same **`deployment_seq`** (from build) are processed in the same wave. Lower sequences complete before higher ones. This respects **`demands`** between job commands (e.g. job B depends on job A).

Reference: [jobs-and-dependencies.md](./jobs-and-dependencies.md) — demand graph, `deployment_seq` algorithm, `update_parallel_count` within a wave, examples.

Within one sequence, jobs are independent except they share worker staging directories under `tmp/workers/<ip>/`.

---

## Which jobs run in a deploy wave

A job is considered only if it appears in the sequence and passes the **`--jobs`** filter.

**`JobNeedsRollout`** decides whether a job is staged and deployed:

| Condition | Action |
|-----------|--------|
| Active allocation has **no hash row** yet | Rollout (first deploy). |
| `previous_hash != current_hash` on an allocation | Rollout (updated content). |
| `previous_hash == current_hash` (promoted after success) | **Skipped** — log: `deploy: skip job "..." (already promoted on all allocations)`. |

After a successful deploy, **`promoteAllocationHash`** sets `previous_hash = current_hash` and **`current_version = new_version`**. A re-run of `maand deploy` therefore **continues from failed jobs only** (partial deploy resume).

---

## Allocation version tracking

Each active allocation tracks **running** vs **target** version alongside content hashes in the **`hash`** table (namespace `<job>_allocation`, key `alloc_id`).

| Field | Meaning |
|-------|---------|
| **`current_version`** | Last **promoted** (running) version on that allocation |
| **`new_version`** | Target version from the current build/deploy plan (`manifest.json` → `job.version`) |

**Defaults:** If `manifest.json` omits **`version`**, maand uses **`0.0.0`** for KV and allocation version fields (build-time dependency rules still require an explicit version when the job participates in the demand graph).

**Lifecycle** (mirrors content hash promote):

```text
First deploy:     current_version=0.0.0   new_version=2.0.0
                  → make restart/start with CURRENT_VERSION=0.0.0 NEW_VERSION=2.0.0
                  → promote → current_version=2.0.0

Upgrade:          current_version=2.0.0   new_version=2.1.0
                  → restart → promote → current_version=2.1.0

Same version, unchanged tree: hash unchanged → job skipped (no restart)
```

During a **rolling deploy**, allocations on different workers can briefly differ (`current_version` updated per allocation as each wave promotes).

**Where to read versions**

| Surface | Keys / fields |
|---------|----------------|
| Job-level KV (target) | `maand cat kv get maand/job/<job> version` |
| Per-allocation KV | `maand/job/<job>/worker/<ip>/current_version`, `new_version` |
| Templates (`.tpl`) | `{{ .CurrentVersion }}`, `{{ .NewVersion }}` on allocation context |
| Worker **`make`** env | `CURRENT_VERSION`, `NEW_VERSION` on `start` / `restart` |
| Job command scripts | Same env vars (plus `NEW_ALLOCATIONS` / `UPDATED_ALLOCATIONS` for `job_control`) |

Example Makefile upgrade hook:

```makefile
restart:
	@echo "Upgrading $(CURRENT_VERSION) -> $(NEW_VERSION)"
	./bin/upgrade.sh
```

Build-time **`version`** rules and demand **`min_version`** / **`max_version`** are separate — see [jobs-and-dependencies.md](./jobs-and-dependencies.md#job-version).

---

Use **`maand deploy --dry-run`** to see whether a real deploy would run, without contacting workers or committing hash updates:

1. Stages job files under `tmp/workers/` (same as deploy).
2. Computes **MD5** of each active allocation’s staged tree and compares to **`previous_hash`** in the database (rolled back afterward).
3. Prints per job whether deploy is required and per allocation whether it would **start**, **restart**, or **skip**.

```bash
maand deploy --dry-run
maand deploy -b -n --jobs api
```

Example output when content changed since last promote:

```text
deploy dry-run: deployment required

deployment sequence 0:
  job "api": deploy required
    10.0.0.1  restart previous_hash=abc... current_hash=def...
```

When everything is promoted:

```text
deploy dry-run: no deployment required

deployment sequence 0:
  job "api": skip (already promoted on all allocations)
```

---

## Default deploy path (Makefile + runner)

For each job in the wave that needs rollout:

1. **`pre_deploy`** (optional)
2. **Stage** job files to `tmp/workers/<ip>/jobs/<job>/`
3. **Rsync** that job to each worker allocation
4. **Update allocation hashes** for that job
5. **Start** new allocations (`previous_hash IS NULL`) or **restart** updated ones (`previous_hash != current_hash`) — immediately after copy
6. **`post_deploy`**, then **promote** hashes for that job

If the job has **no** `job_control` commands:

1. **`handleNewAllocations`**: Workers where `previous_hash IS NULL` (new alloc) →  
   `python3 /opt/worker/<bucket_id>/bin/runner.py <bucket_id> start --jobs <job>`
2. **`handleUpdatedAllocations`**: Workers where hash changed →  
   `runner.py ... restart --jobs <job>` in batches of **`update_parallel_count`**.
3. **`health_check`**: If the job has `health_check` commands, run them with **wait/retry** (same as deploy-internal health check).
4. **`post_deploy`**: Job commands with event `post_deploy`.
5. **`promoteAllocationHash`**: Mark current tree and **`current_version`** as the new baseline.

**Makefile** on the worker (under `jobs/<job>/`) receives **`CURRENT_VERSION`** and **`NEW_VERSION`** in the environment for **`start`** and **`restart`**. Use them for upgrade logic (see [Allocation version tracking](#allocation-version-tracking)):

```makefile
start:
stop:
restart:
```

Data/logs/bin on workers are excluded from rsync (`--exclude=jobs/*/data`, etc.).

---

## `job_control` deploy path

If the manifest registers any command with **`executed_on` including `job_control`**:

- Default start/restart/stop is **not** used.
- Each `job_control` command runs via **`jobcommand.JobCommand`** with extra env:
  - `NEW_ALLOCATIONS=<comma-separated IPs>`
  - `UPDATED_ALLOCATIONS=<comma-separated IPs>`
  - `CURRENT_VERSION`, `NEW_VERSION` (per allocation)
- Then **health_check** (with wait), **post_deploy**, **promote**.

You implement rollout logic inside your scripts.

---

## Staging and rsync

### Staging (`tmp/workers/<worker_ip>/`)

| Path | Content |
|------|---------|
| `worker.json` | `bucket_id`, `worker_id`, labels, `update_seq` |
| `jobs.json` | Active jobs on this worker (removed jobs omitted) |
| `bin/runner.py`, `bin/worker.py` | Embedded helpers |
| `jobs/<job>/` | Copy of `job_files` + rendered `.tpl` + `certs/` |

### Rsync

- From bucket container to **`agent@<worker>:/opt/worker/<bucket_id>/`** (user from `maand.conf`).
- **Staging rsync**: filter includes only jobs in **`jobsToStage`** (`+ jobs/<job>/`, `- jobs/*`).
- **Final rsync**: one pass **per successfully deployed job**, only to workers with an **active** allocation for that job; same per-job filter so other jobs on the host are not touched.

### Templates (`.tpl`)

Files under `jobs/<job>/` ending in **`.tpl`** are rendered with Go templates. Functions: `get`, `getSecret`, `keys`, `split`, `join`, etc. Use **`getSecret "key"`** for values written with **`put_job_secret`** under `secrets/job/<job>`.

Allocation template fields include **`CurrentVersion`** (running) and **`NewVersion`** (deploy target). Equivalent KV keys: `get "maand/job/<job>/worker/<ip>" "current_version"` and `"new_version"`.

---

## Removed and disabled allocations

| State | Behavior |
|-------|----------|
| **`removed=1`** (worker/job dropped at build) | If previously deployed: **stop**, then **`rm -rf`** `jobs/<job>/` on the worker (and local staging). Workers removed from **`workers.json`**: after all their removed allocations are processed, **`rm -rf /opt/worker/<bucket_id>/`**. Unreachable removed workers are assumed dead (logged, deploy continues). |
| **`disabled=1`** | Excluded from start/restart/rsync targets; stop if was running; remove job dir; promote clears `previous_hash` and **`current_version`** for disabled row. |

---

## `pre_deploy` and `post_deploy`

- Registered in manifest with `executed_on`: `pre_deploy` / `post_deploy`.
- Run via **`jobcommand`** inside the maand container (Python or Bun).
- **`pre_deploy` failure**: job is not added to `jobsToStage` for this deploy; other jobs continue.
- **`post_deploy` failure**: fails that job’s deploy; earlier jobs in the same run may already be promoted.

### KV checkpoint

Job commands can write to the in-memory KV store (e.g. connection strings for templates). After each job’s **`pre_deploy`** and after **`deployJob`** (including `post_deploy`), maand flushes pending KV changes into the **deploy transaction** via **`kv.PersistToSessionTransaction`**.

Those writes commit when deploy commits (including **partial deploy** — successful jobs persist even if later jobs fail). If deploy aborts before commit, KV checkpoint writes roll back with the rest of the catalog state.

---

## Partial deploy and retry

1. Job A succeeds → content hashes and **`current_version`** **promoted**.
2. Job B fails (e.g. `post_deploy` or restart error).
3. Transaction still **commits**; command returns **`deploy failed`** with errors.
4. Fix B, run **`maand deploy`** again:
   - Job A: **skipped** (already promoted).
   - Job B: staged and deployed again.

---

## `update_seq`

At deploy start, **`bucket.update_seq`** increments in its own committed transaction. Workers receive the new value in **`worker.json`** so they can detect bucket-wide changes.

---

## Configuration

Uses **`workspace/maand.conf`** (SSH user, key file name under `secrets/`, sudo for remote rsync).

Worker key path inside container: `/bucket/secrets/<ssh_key>`.

---

## Inspect state

```bash
maand cat allocations
maand cat jobs
maand info
```

Hash state lives in table **`hash`** with namespace `<job>_allocation` and key `alloc_id`.

---

## Common failures

| Symptom | Likely cause |
|---------|----------------|
| SSH / rsync errors | Wrong key, firewall, worker down, sudo needed (`use_sudo=true`) |
| `deploy: skip job` on first deploy | Run `build` first; allocation should have no hash row until first successful promote |
| `deploy: skip job` after workspace edit | Run **`maand build`** then **`maand deploy`**; deploy refreshes plan hashes from staged content before the skip check |
| Job not staging | `pre_deploy` failed or `JobNeedsRollout` false |
| Template panic | KV key missing; namespace not allowed in `.tpl` |
| `health check failed` | `health_check` command returned non-zero |
| Stale files on worker | Removed job dirs should be pruned; final rsync is per-job only |

---

## Related commands

- [`build.md`](./build.md) — catalog and sequences.
- [`job-command.md`](./job-command.md) — writing `command_*` scripts.
- [`health-check.md`](./health-check.md) — standalone health checks.
- `maand job start|stop|restart|run|status` — manual job control ([job.md](./job.md)).
- `maand gc` — purge removed allocations and worker runtime dirs ([gc.md](./gc.md)).
