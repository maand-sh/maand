# Disabling workers, jobs, and allocations

Use **`workspace/disabled.json`** to stop workloads **without** removing workspace files, catalog rows, or worker **`data/`** / **`logs/`**. Disabled allocations are **inactive** for start/restart/rsync, but maand still **stages, hashes, and promotes** content and version changes on deploy so you can re-enable safely after a build.

For removal (soft-delete until GC), see [gc.md](./gc.md) and [day-2-operations.md](./tutorials/day-2-operations.md#remove-a-worker-or-job).

---

## `disabled.json` format

File path: **`workspace/disabled.json`** (optional; missing file means nothing disabled).

```json
{
  "jobs": {
    "api": {},
    "vault": {
      "allocations": ["10.0.0.2"]
    }
  },
  "workers": ["10.0.0.3"]
}
```

| Key | Effect |
|-----|--------|
| **`jobs.<name>`** with empty object `{}` | Disable **every** allocation for that job (all workers). |
| **`jobs.<name>.allocations`** | Disable only those worker IPs for that job. |
| **`workers`** | Disable **every** job on those worker IPs. |

Rules:

- Job and worker entries can be combined in one file.
- A worker listed under **`workers`** disables all its allocations, even if not listed under **`jobs`**.
- Edit **`disabled.json`**, then always run **`maand build`** so `allocations.disabled` updates in `maand.db`.

---

## How to disable

### One allocation (job on one worker)

Drain **`api`** on **`10.0.0.2`** only:

```json
{
  "jobs": {
    "api": {
      "allocations": ["10.0.0.2"]
    }
  }
}
```

```bash
maand build
maand deploy
```

Deploy **stops** the job on that worker if it was running. Other workers keep running **`api`**.

### Entire job (all workers)

```json
{
  "jobs": {
    "api": {}
  }
}
```

```bash
maand build
maand deploy
```

Every **`api`** allocation is stopped; deploy skips start/restart/rsync for **`api`**.

### Entire worker (all jobs on a host)

```json
{
  "workers": ["10.0.0.3"]
}
```

```bash
maand build
maand deploy
```

Use this before host maintenance: drain all jobs on the machine without deleting job definitions.

### Combine job and worker rules

```json
{
  "jobs": {
    "vault": {
      "allocations": ["10.0.0.1"]
    }
  },
  "workers": ["10.0.0.4"]
}
```

---

## What happens on build and deploy

| Phase | Disabled allocation |
|-------|---------------------|
| **`maand build`** | Sets `allocations.disabled = 1`. Re-enable clears the flag when the entry is removed from `disabled.json`. |
| **`maand deploy` reconcile** | **Stop** if previously deployed (`previous_hash` set). **Keep** deploy artifacts, **`data/`**, **`logs/`**, hash row, and job KV. |
| **Staging / hash / promote** | Still runs for non-removed allocations (including disabled). Content and **`new_version`** stay current in the catalog. |
| **Start / restart / rsync** | Skipped for disabled rows. |
| **`maand job start`** | Skipped unless you target active allocations only. |

Inspect state:

```bash
maand cat allocations --jobs api
maand cat hashes --jobs api
```

**`cat hashes` rollout** for disabled rows:

| Rollout | Meaning |
|---------|---------|
| `disabled` | Stopped; hashes and versions match (promoted). |
| `disabled_restart` | Stopped; content or version changed since last promote — deploy updated the plan but did not restart. |

---

## How to re-enable

1. Remove the job, allocation, or worker entry from **`disabled.json`** (or replace with `{}` / delete the file).
2. **`maand build`** — clears `disabled = 0` and marks the allocation for **start** on the next deploy.
3. **`maand deploy`** — runs **`make start`** (or **`job_control`**) on re-enabled workers.

```bash
# disabled.json no longer lists api on 10.0.0.2
maand build
maand deploy
maand job status api --allocations 10.0.0.2   # optional check
```

Re-enable does **not** require a workspace content change. Hash and KV from before disable are reused.

---

## Disabled vs removed

| | **Disabled** | **Removed** |
|--|--------------|-------------|
| Trigger | `disabled.json` | Drop worker from **`workers.json`** or delete **`workspace/jobs/<job>/`** |
| Catalog row | Kept (`removed=0`) | Kept until **`maand gc`** (`removed=1`) |
| Deploy artifacts on worker | **Kept** | **Removed** ( **`data/`** / **`logs/`** kept until GC) |
| Hash row | **Kept** | **Deleted** on deploy reconcile |
| Job KV | **Kept** | Purged when no non-removed allocations remain |
| Typical use | Maintenance, drain, pause | Decommission job or worker |

Workflow for **removal**:

```bash
maand build
maand deploy
maand gc
```

---

## Operations that skip disabled allocations

These target **active** allocations only (`removed=0`, `disabled=0`):

- Default deploy start/restart and per-job rsync
- **`maand job_command`** (unless you use hooks that run at build/deploy catalog level)
- **`maand health_check`** default job list (use **`--jobs`** explicitly if needed)

These still see disabled rows where relevant:

- **`maand cat allocations`**, **`maand cat hashes`** (use **`--active`** to filter)
- **`maand cat kv --jobs <job>`** — includes namespaces for disabled-but-not-removed allocations
- Deploy **hash refresh** and **promote** for disabled jobs (catalog stays current)

---

## Examples

### Maintenance window on one worker

```bash
# 1. Drain
cat > workspace/disabled.json <<'EOF'
{ "workers": ["10.0.0.3"] }
EOF
maand build && maand deploy

# 2. Patch host (reboot, kernel, etc.) — see rolling-upgrade.md

# 3. Re-enable
echo '{}' > workspace/disabled.json   # or remove workers entry
maand build && maand deploy
```

### Pause a job globally; keep shipping config

While **`api`** is disabled, you can still edit **`workspace/jobs/api/`**, run **`maand build`**, and **`maand deploy`**. Catalog hashes and versions update; workers stay stopped until you clear **`disabled.json`** and deploy again.

### Verify disable took effect

```bash
maand cat allocations --jobs api
# disabled=1 on target rows

maand cat hashes --jobs api
# rollout=disabled or disabled_restart

maand deploy --dry-run
# re-enabled rows may show start; disabled rows omitted from active rollout
```

---

## Related

- [concepts.md](./concepts.md) — active vs inactive allocations
- [deploy.md](./deploy.md#removed-and-disabled-allocations) — reconcile behavior
- [build.md](./build.md) — `disabled.json` during build
- [rolling-upgrade.md](./rolling-upgrade.md) — rolling restarts and worker reboots
- [deploy-debugging.md](./deploy-debugging.md) — when disable/re-enable does not behave as expected
