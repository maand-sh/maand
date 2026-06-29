# 8. Deploy

**`maand deploy`** pushes job trees to workers and runs lifecycle Makefile targets.

```bash
maand deploy
maand deploy --jobs api
maand deploy --dry-run
maand deploy --force --jobs api
```

Prerequisites: successful **`maand build`**, SSH access, host tools (`ssh`, `rsync`, `python3`).

---

## Deploy phases (simplified)

```text
For each deployment_seq wave (dependency order):
  For each job in the wave:
    reconcile   → stop removed/disabled allocations
    pre_deploy  → job command hook (CLI host)
    rsync       → stage tmp/workers/<ip>/ → /opt/worker/<bucket_id>/
    rollout     → make start | restart | reload per allocation
    post_deploy → job command hook
    promote     → update hash / version in catalog
```

Rolling happens **within** a job using batch sizes from the manifest.

---

## First deploy vs upgrade

| Allocation state | After rsync | Lifecycle |
|------------------|-------------|-----------|
| New (no promoted hash) | Files on worker | **`make start`** |
| Existing, content or version changed | Updated files | **`make restart`** or **`make reload`** per policy |
| Unchanged hash + version | — | **skip** (unless `--force`) |

Inspect skip vs rollout:

```bash
maand deploy --dry-run
maand cat deployments --jobs api
```

---

## `restart_policy`

Set in `manifest.json`:

| Policy | On upgrade |
|--------|------------|
| **`always`** (default) | `make restart` |
| **`reload`** | `make reload`, unless a changed file matches **`restart_globs`** → then `restart` |
| **`never`** | rsync only; no lifecycle |

**Config-only push** without lifecycle for one deploy:

```bash
maand deploy --sync-only --jobs api
```

Then manually: `maand job run api --target reload` if needed.

Reference: [deploy.md](../reference/cli/deploy.md#applying-changes-on-workers).

---

## Rolling batches

| Manifest field | Controls |
|----------------|----------|
| `deploy_parallel_count` | Batch size for **`start`** on first rollout |
| `update_parallel_count` | Batch size for **`restart`** / **`reload`** on upgrades |

Within each batch, order comes from **`deploy_order`** (KV key, overridable in `pre_deploy`).

Guide: [rolling-deploy.md](../guides/rolling-deploy.md).

---

## Deploy hooks (preview)

| Event | Runs on | Typical use |
|-------|---------|-------------|
| `pre_deploy` | CLI host, per allocation | Secrets, `put_deploy_order`, migrations prep |
| `post_deploy` | CLI host, per allocation | Notify, KV updates |
| `after_allocation_started` | CLI host | Post-start validation |
| `job_control` | CLI host | Replace default start/restart with custom script |

Details: [10-job-commands.md](./10-job-commands.md).

---

## Version tracking

Each allocation tracks:

- **`current_version`** — running (in `hash` table after promote)
- **`new_version`** — target from manifest (in `allocations` row)

A version bump can trigger rollout even when file hash is unchanged. Makefile gets `CURRENT_VERSION` and `NEW_VERSION`.

---

## When deploy goes wrong

```bash
maand deploy --dry-run
maand cat deployments
maand logs show --job api --format human
```

Guide: [debugging-deploy.md](../guides/debugging-deploy.md).

---

## Next

[09 — Configuration, KV, and templates](./09-configuration-kv-and-templates.md).
