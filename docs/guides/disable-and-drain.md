# Disabling workers, jobs, and allocations

Use **`workspace/disabled.json`** for **maintenance** — drain a node, allocation, or entire job **without** removing workspace files, catalog rows, or worker **`data/`** / **`logs/`**.

A disabled allocation is otherwise the same as an active one: it stays in **`allocations`**, build refreshes job and per-allocation KV and certs, and deploy **stages, hashes, and promotes** content. The **only** difference is runtime: maand **never starts** a disabled allocation (no lifecycle — start, restart, reload — and no rsync on deploy).

For removal (soft-delete until GC), see [gc.md](../reference/cli/gc.md) and [day-2-ops.md](day-2-ops.md#remove-a-worker-or-job).

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

Every **`api`** allocation is stopped; deploy skips lifecycle and rsync for **`api`**.

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
| **`maand build`** | Sets `allocations.disabled = 1`. Same job KV, per-allocation KV (`*_allocation_index`, `peer_workers`), and certs as active peers. Re-enable clears the flag when the entry is removed from `disabled.json`. |
| **`maand deploy` reconcile** | **Stop** if previously deployed (`previous_hash` set). **Keep** deploy artifacts, **`data/`**, **`logs/`**, hash row, and KV. |
| **Staging / hash / promote** | Same as active — content and **`new_version`** stay current. |
| **Lifecycle / rsync** | **Skipped** (no start, restart, reload, or rsync) — the only runtime difference from active. |
| **`maand job start`** | Skipped unless you target active allocations only. |

Inspect state:

```bash
maand cat allocations --jobs api
maand cat deployments --jobs api
maand deploy --dry-run
```

After disable, **`maand deploy --dry-run`** should show **`stop`** on allocations that were running (rollout `disabled` in **`cat deployments`**). If build changed content or version while disabled, dry-run shows **`stop+promote`** or **`promote`** (`disabled_restart` in **`cat deployments`**) — deploy stages and promotes without starting the allocation.

**`cat deployments` rollout** for disabled rows:

| Rollout | Meaning |
|---------|---------|
| `disabled` | Stopped; hashes and versions match (promoted). |
| `disabled_restart` | Stopped; content or version changed since last promote — deploy updated the plan but did not restart. |

---

## How to re-enable

1. Remove the job, allocation, or worker entry from **`disabled.json`** (or replace with `{}` / delete the file).
2. **`maand build`** — clears `disabled = 0` and marks the allocation for rollout on the next deploy.
3. **`maand deploy`** — runs the planned lifecycle (**`make start`**, **`restart`**, or **`reload`** per **`restart_policy`**, or **`job_control`**) on re-enabled workers.

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

- Default deploy lifecycle (start / restart / reload) and per-job rsync
- **`maand job_command`** (unless you use hooks that run at build/deploy catalog level)
- **`maand health_check`** default job list (use **`--jobs`** explicitly if needed)

These still see disabled rows where relevant:

- **`maand cat allocations`**, **`maand cat deployments`** (use **`--active`** to filter)
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

maand cat deployments --jobs api
# rollout=disabled or disabled_restart

maand deploy --dry-run
# re-enabled rows may show start; disabled rows omitted from active rollout
```

---

## Related

- [concepts.md](../start/concepts.md) — active vs inactive allocations
- [deploy.md](../reference/cli/deploy.md#removed-and-disabled-allocations) — reconcile behavior
- [build.md](../reference/cli/build.md) — `disabled.json` during build
- [rolling-deploy](rolling-deploy.md) — rolling restarts and worker reboots
- [debugging-deploy.md](debugging-deploy.md) — when disable/re-enable does not behave as expected
