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
| `--force` | | Redeploy jobs even when all allocations are already promoted (restart active allocations). |
| `--sync-only` | | Rsync and promote without `start` / `restart` / `reload`. **Fails** when any allocation still needs **`start`** (new allocation). |

Examples:

```bash
maand deploy
maand deploy -b
maand deploy --jobs api,worker
maand deploy --dry-run
maand deploy -b -n
maand deploy --force --jobs vault
maand deploy --sync-only --jobs prometheus
maand deploy --dry-run --sync-only
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
Stop removed/disabled allocations; remove deployed job files from **removed** workers only (preserve data/logs; disabled keeps artifacts; full bucket tree when the worker left workers.json)
For deployment_seq = 0 .. max:
  Prepare worker.json / jobs.json / bin/ for all workers
  Refresh plan hashes for jobs in this sequence
  For each job needing rollout:
    pre_deploy commands → KV checkpoint (into deploy transaction)
  For each job to stage:
    prepare job files + transpile .tpl + certs → tmp/workers/<ip>/jobs/<job>/
    rsync (filtered per job) to /opt/worker/<bucket_id>/
    update allocation content hashes (MD5 of staged tree)
    deployJob (start/restart/reload OR job_control + health_check + post_deploy)
    KV checkpoint again
Final rsync: per successfully deployed job only (filtered + jobs.json refresh)
Commit transaction (even if some jobs failed — partial deploy)
Return joined errors if any job failed
```

---

## Deployment sequence

Jobs with the same **`deployment_seq`** (from build) are processed in the same wave. Lower sequences complete before higher ones. This respects **`demands`** between job commands (e.g. job B depends on job A).

Reference: [deployment-sequence.md](../deployment-sequence.md) — demand graph, `deployment_seq` algorithm, `max_concurrent_upgrades` within a wave, examples.

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
| **`--force`** | Stage and **restart** all active allocations (except new ones, which still **start**). |

After a successful deploy, **`promoteAllocationHash`** sets `previous_hash = current_hash` and **`hash.current_version = allocations.new_version`**. A re-run of `maand deploy` therefore **continues from failed jobs only** (partial deploy resume). Use **`--force`** to roll the same content again without a workspace change.

---

## Allocation version tracking

Each active allocation tracks **running** vs **target** version alongside content hashes in the **`hash`** table (namespace `<job>_allocation`, key `alloc_id`).

| Field | Meaning |
|-------|---------|
| **`current_version`** (`hash` table) | Last **promoted** (running) version on that allocation |
| **`new_version`** (`allocations` table) | Target version from **`maand build`** (`manifest.json` → `job.version`) |

**Defaults:** If `manifest.json` omits **`version`**, maand uses **`0.0.0`** for KV and allocation version fields (build-time dependency rules still require an explicit version when the job participates in the demand graph).

**Lifecycle** (mirrors content hash promote):

```text
First deploy:     current_version=0.0.0   new_version=2.0.0
                  → make start with CURRENT_VERSION=0.0.0 NEW_VERSION=2.0.0
                  → promote → current_version=2.0.0

Upgrade:          current_version=2.0.0   new_version=2.1.0
                  → restart/reload (per restart_policy) → promote → current_version=2.1.0

Same version, unchanged tree: hash unchanged and `current_version = new_version` → job skipped (no lifecycle)

Version-only bump: **`build`** updates `allocations.new_version`; **`deploy`** runs lifecycle when `hash.current_version != allocations.new_version` even if the content hash is unchanged (typically **`reload`** when policy is **`reload`**)
```

During a **rolling deploy**, allocations on different workers can briefly differ (`current_version` updated per allocation as each wave promotes).

**Where to read versions**

| Surface | Keys / fields |
|---------|----------------|
| Job-level KV (target) | `maand cat kv get maand/job/<job> version` |
| Per-allocation KV | `maand/job/<job>/worker/<ip>/version` (target from build) |
| Templates (`.tpl`) | `{{ .CurrentVersion }}`, `{{ .NewVersion }}` on allocation context |
| Worker **`make`** env | `CURRENT_VERSION`, `NEW_VERSION` on `start` / `restart` / `reload` |
| Job command scripts | Same env vars (plus `NEW_ALLOCATIONS` / `UPDATED_ALLOCATIONS` for `job_control`) |

Example Makefile upgrade hook:

```makefile
restart:
	@echo "Upgrading $(CURRENT_VERSION) -> $(NEW_VERSION)"
	./bin/upgrade.sh
```

Build-time **`version`** rules and demand **`min_version`** / **`max_version`** are separate — see [manifest.md](../manifest.md#version).

---

Use **`maand deploy --dry-run`** to see whether a real deploy would run, without rsync, lifecycle, or hash promotion:

1. Stages job files under `tmp/workers/` (same as deploy), including **`pre_deploy`** hooks when registered (so plan hashes match real deploy staging). **`pre_deploy` may SSH to workers** when the hook runs job commands on allocations.
2. Computes **MD5** of each active allocation’s staged tree and compares to **`previous_hash`** in the database (rolled back afterward).
3. Prints per job whether deploy is required and per allocation the planned action: **start**, **restart**, **reload**, **sync**, or **skip**. When **`restart_globs`** forces a restart, the line includes **`matched=`** with the changed paths.

```bash
maand deploy --dry-run
maand deploy -b -n --jobs api
maand deploy --dry-run --force    # preview forced redeploy
```

Example output when content changed since last promote:

```text
deploy dry-run: deployment required

deployment sequence 0:
  job "api": deploy required
    10.0.0.1  reload  previous_hash=abc... current_hash=def...
    10.0.0.2  restart previous_hash=abc... current_hash=def... matched=Makefile
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
5. **Start** new allocations (`previous_hash IS NULL`) or apply **`restart_policy`** on updated ones — immediately after copy
6. **`post_deploy`**, then **promote** hashes for that job

If the job has **no** `job_control` commands:

1. **`handleNewAllocations`**: Workers where `previous_hash IS NULL` (new alloc) →  
   `python3 /opt/worker/<bucket_id>/bin/runner.py <bucket_id> start --jobs <job>`  
   in batches of **`max_concurrent_starts`** (0 = all at once), ordered by **`rollout_order`**.  
   **`after_allocation_started`** hooks run after each batch. **One** health check runs after all start batches complete.
2. **`handleUpdatedAllocations`**: Workers where hash or version changed → lifecycle per **`restart_policy`** (see below) in batches of **`max_concurrent_upgrades`**, ordered by **`rollout_order`**.  
   **`after_allocation_started`** hooks run after each batch, then **health_check** (wait/retry) before the next batch when a lifecycle target runs.
3. **`post_deploy`**: Job commands with event `post_deploy`.
4. **`promoteAllocationHash`**: Mark current tree and **`current_version`** as the new baseline.

When allocations are **stopped** during reconcile (removed/disabled), **`after_allocation_stopped`** hooks run once per stopped allocation before the default Makefile stop.

**Makefile** on the worker (under `jobs/<job>/`) receives **`CURRENT_VERSION`** and **`NEW_VERSION`** in the environment for **`start`**, **`restart`**, and **`reload`**. Use them for upgrade logic (see [Allocation version tracking](#allocation-version-tracking)):

```makefile
start:
stop:
restart:
reload:
```

Data/logs/bin on workers are excluded from rsync (`--exclude=jobs/*/data`, etc.).

---

## Applying changes on workers

Deploy always **rsyncs** staged files before it decides whether to touch running processes. That split matters: you can push config to disk while choosing how (or whether) the process reacts.

### What triggers rollout

| Situation | Typical action |
|-----------|----------------|
| First deploy on a worker (`previous_hash` empty) | **`make start`** |
| Staged tree differs from last promote (`previous_hash ≠ current_hash`) | Lifecycle per **`restart_policy`** (below) |
| Version target changed (`current_version ≠ new_version`, same tree) | Same lifecycle policy (usually **`reload`** when policy is `reload`) |
| Already promoted on all active allocations | **Skip** — no rsync wave for that job |
| **`--force`** | Rollout all active allocations even when hash and version match |

**New allocations always start.** No policy or flag can replace `start` with rsync-only — use normal deploy for first boot, then tune policy for upgrades.

### Default path: Makefile + `runner.py`

When the job has no **`job_control`** commands, deploy calls **`runner.py`** on the worker, which runs **`make`** targets in the job directory:

| Target | When |
|--------|------|
| **`start`** | New allocation |
| **`restart`** | Full recreate / stop-start (policy **`always`**, or **`reload`** + **`restart_globs`** match) |
| **`reload`** | Soft apply — config reload, HTTP `/-/reload`, `systemctl reload`, etc. |

Each target receives **`CURRENT_VERSION`** and **`NEW_VERSION`** in the environment (see [Allocation version tracking](#allocation-version-tracking)).

Rolling batches use **`max_concurrent_starts`** (starts) and **`max_concurrent_upgrades`** (restarts/reloads), ordered by **`rollout_order`**. Health checks run after start batches complete and after each update batch when a lifecycle target runs.

### `restart_policy` (manifest)

Set in **`manifest.json`**. Default **`always`**. Applies to **updated** allocations only.

| Value | After rsync | Makefile |
|-------|-------------|----------|
| **`always`** | Recreate or full restart | `make restart` |
| **`reload`** | Soft apply when possible | `make reload` (see **`restart_globs`**) |
| **`never`** | Files only | — (rsync + promote) |

Example — Prometheus picks up rule and config changes without restarting the process when only non-critical files change:

```json
{ "restart_policy": "reload" }
```

```makefile
reload:
	curl -sf -X POST http://127.0.0.1:$(PROM_PORT)/-/reload
```

Prometheus needs **`--web.enable-lifecycle`**. See [guides/prometheus.md](../../guides/prometheus.md).

Stateful jobs (databases, queues) usually keep **`always`**. Stateless HTTP services and monitoring stacks often use **`reload`**.

### `restart_globs` (manifest, with `reload` only)

Optional list of **job-relative globs** (`*`, `?`, `**` — same rules as `.dashboardignore`). **`maand build`** rejects **`restart_globs`** unless **`restart_policy`** is **`reload`**.

Maand stores a **per-file manifest** on each allocation (`hash.current_files` / `hash.previous_files`): path → content MD5 of the last staged and promoted trees. On upgrade it diffs those maps:

- If **any changed path** matches a glob → **`make restart`**
- Otherwise → **`make reload`**

| Changed files | `restart_globs` | Result |
|---------------|-----------------|--------|
| `rules/alerts.yaml` | `["prometheus.yml", "Makefile"]` | reload |
| `Makefile` | same | restart |
| Version bump only (no file diff) | any | reload |
| No promoted file manifest yet | any | reload (conservative default) |

Example — reload for most edits, restart when compose or binaries change:

```json
{
  "restart_policy": "reload",
  "restart_globs": [
    "docker-compose.yml",
    "docker-compose.yml.tpl",
    "Dockerfile",
    "bin/**"
  ]
}
```

Dry-run shows which paths triggered restart:

```text
10.0.0.3  restart previous_hash=... current_hash=... matched=Makefile,bin/app
10.0.0.4  reload  previous_hash=... current_hash=...
```

### `--sync-only` (CLI)

One-deploy override: **rsync**, **`post_deploy`**, and **promote** without **`start`**, **`restart`**, or **`reload`**. Same effect as **`restart_policy: never`** for updated allocations, but chosen on the command line.

| Case | Behavior |
|------|----------|
| Updated allocation | Rsync + promote; no lifecycle |
| New allocation | **Error** — cannot bootstrap without **`start`** |
| **`job_control`** job | Skips custom lifecycle; rsync + promote only |
| **`--dry-run`** | Reports **`sync`**; errors if **`start`** would be required |

Use when the process reads files directly, or when you will run **`maand job run <job> --target reload`** yourself afterward.

```bash
maand build
maand deploy --dry-run --sync-only --jobs api
maand deploy --sync-only --jobs api
```

**`--force --sync-only`** still skips lifecycle even when force would otherwise roll allocations.

### `job_control` (custom lifecycle)

If the manifest registers **`job_control`**, default **`start` / `restart` / `reload`** are **not** used. Your script receives **`NEW_ALLOCATIONS`**, **`UPDATED_ALLOCATIONS`**, **`CURRENT_VERSION`**, and **`NEW_VERSION`** — implement canary, blue/green, or sync logic there. See [job-command-api.md](../job-command-api.md).

**`restart_policy`**, **`restart_globs`**, and **`--sync-only`** do not apply on this path (except **`--sync-only`** still skips the script and only rsyncs + promotes).

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

- From the bucket directory on the CLI host to **`agent@<worker>:/opt/worker/<bucket_id>/`** (user from `maand.conf`).
- **Staging rsync**: filter includes only jobs in **`jobsToStage`** (`+ jobs/<job>/`, `- jobs/*`).
- **Final rsync**: one pass **per successfully deployed job**, only to workers with an **active** allocation for that job; same per-job filter so other jobs on the host are not touched.

### Templates (`.tpl`)

Files under `jobs/<job>/` ending in **`.tpl`** are rendered at staging time. Full reference: [templates.md](../templates.md).

### Prometheus job staging

When the staged job ships **`prometheus.yml`** or **`prometheus.yml.tpl`**, maand assembles monitoring artifacts **before** rsync (from **`job_files`**, not from `maand/prometheus` KV except scrape expansion):

| Output (under `jobs/prometheus/` on worker) | Source |
|---------------------------------------------|--------|
| `rules/<maand_job>/*.yaml` | Each job's `_prometheus/alerts/` (+ runbook URL injection) |
| `rules/maand/certs.yaml` | Embedded cert alert rules when server config exists |
| `consoles/runbooks/<job>/<slug>.html` | `_prometheus/runbooks/*.md` → HTML + index + CSS |
| `consoles/dashboards/<job>/<path>` | `_prometheus/dashboards/**` copied as-is (+ index, CSS) |
| `prometheus.yml` (rendered) | Template with `{{ scrapeConfigs }}` / `{{ ruleFiles }}` |

**`{{ scrapeConfigs }}`** reads scrape KV (`maand/prometheus/scrape*`), expands `maand:port/*` using **active** allocations, and **skips** jobs that would expand to zero targets (does not fail the whole render).

After deploy **commits**, maand **best-effort** pushes cert expiry metrics via Prometheus remote write (see [certs.md](../certs.md#prometheus-metrics-optional)).

Details: [prometheus.md](../../guides/prometheus.md).

---

## Removed and disabled allocations

| State | Behavior |
|-------|----------|
| **`removed=1`** (worker/job dropped at build) | If previously deployed: **stop**, then remove deployed job files on the worker (**`data/`** and **`logs/`** are left in place for redeploy). Local staging under `tmp/workers/<ip>/jobs/<job>/` is removed. Workers removed from **`workers.json`**: after all their removed allocations are processed, **`rm -rf /opt/worker/<bucket_id>/`**. Unreachable removed workers are assumed dead (logged, deploy continues). |
| **`disabled=1`** | Excluded from start/restart/reload/rsync targets; stop if was running; **keep** deployed job files, KV, and hash/version state. Content and version changes are still **staged, hashed, and promoted** on deploy (rollout shows `disabled` or `disabled_restart` in `maand cat deployments`). After re-enable (`maand build` clears `disabled.json`), deploy **starts** the allocation via `GetNewAllocations`. |

Redeploying the same job on the same worker reuses existing **`data/`** and **`logs/`** (rsync excludes those paths). **`deploy`** deletes the allocation **hash row** when reconciling **`removed=1`** allocations (even if the job is skipped from rollout), so a later redeploy treats it as a **new** rollout (`make start`) while worker **`data/`** and **`logs/`** remain. After **`build`** only, hashes still show the last promoted state until **`deploy`** runs.

When reconcile finishes and a job has **no non-removed allocations** (every row `removed=1`), deploy purges all job-scoped KV namespaces. Jobs with **disabled-only** allocations retain KV. **`maand build`** also clears build-owned namespaces when the job is inactive. Run **`maand gc`** to delete worker **`jobs/<job>/`** trees and purge removed allocation rows from the catalog.

---

## `pre_deploy` and `post_deploy`

- Registered in manifest with `executed_on`: `pre_deploy` / `post_deploy`.
- Run via **`jobcommand`** on the CLI host (Python or Bun).
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

Use **`maand deploy --force`** to redeploy promoted jobs without a workspace change.

---

## `update_seq`

At deploy start, **`bucket.update_seq`** increments in its own committed transaction. Workers receive the new value in **`worker.json`** so they can detect bucket-wide changes.

---

## Configuration

Uses **`maand.conf`** at the bucket root (SSH user, key file under `secrets/`, sudo for remote rsync). See [configuration.md](../configuration.md).

Worker key path: **`secrets/<ssh_key>`** relative to the bucket root.

---

## Inspect state

```bash
maand cat allocations
maand cat deployments
maand cat jobs
maand info
```

Hash state lives in table **`hash`** with namespace `<job>_allocation` and key `alloc_id`. **`maand cat deployments`** shows **`current_hash`**, **`previous_hash`**, versions, and rollout (`removed`, `disabled`, or hash-derived `new` / `restart` / `promoted` / `health_failed`). Use **`--active`** to see only allocations deploy would target.

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
| Stale files on worker | Deploy removes deployed job files on dealloc (keeps `data/`/`logs/`); GC deletes runtime dirs; final rsync is per-job only |

---

## Related

- [build.md](build.md) — catalog and sequences
- [templates.md](../templates.md) — `.tpl` rendering
- [KV namespaces](../kv/namespaces.md) · [KV persistence](../kv/persistence.md)
- [job-command.md](job-command.md) · [job-command-api.md](../job-command-api.md)
- [disable and drain](../../guides/disable-and-drain.md) — disable/re-enable
- [rolling-deploy](../../guides/rolling-deploy.md) — `max_concurrent_upgrades`, version upgrades
- [debugging-deploy.md](../../guides/debugging-deploy.md) — dry-run, `cat deployments`, failures
- [health-check.md](health-check.md) — standalone health checks
- [job.md](job.md) — manual start/stop/restart/reload
- [gc.md](gc.md) — purge removed allocations
