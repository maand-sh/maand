# `maand gc`

**GC** removes soft-deleted catalog rows, KV references for removed allocations, and old KV history from `maand.db`. It also deletes the full **`jobs/<job>/`** tree on workers for allocations still marked `removed = 1`.

Run **`maand build`** after removing workers or jobs in the workspace so allocations are marked `removed = 1`. **`maand deploy`** stops removed allocations and removes deployed job files from workers (**`data/`** and **`logs/`** are preserved). **`maand gc`** is when worker **`data/`**, **`logs/`**, and **`bin/`** are deleted. Deploy drops the full **`/opt/worker/<bucket_id>/`** tree only for hosts removed from **`workers.json`**.

## CLI

```bash
maand gc [--retain-days N]
```

| Flag | Description |
|------|-------------|
| `--retain-days` | Days to retain deleted KV versions before purge (default `0`). |

## What it does

1. For each allocation still marked `removed = 1`: SSH to the worker and `rm -rf` `/opt/worker/<bucket_id>/jobs/<job>/` (entire job directory). Workers no longer in **`workers.json`** are assumed dead when unreachable.
2. Purge KV namespaces for removed allocations (`maand/job/<job>/worker/<ip>`, and `maand/worker/<ip>` / tags when the worker is off-catalog). When a job has **no active allocations**, also purge all job-level namespaces (`vars/job/<job>`, `secrets/job/<job>`, `maand/job/<job>`, `vars/bucket/job/<job>`). **`maand deploy`** purges the same job-level namespaces when reconcile leaves no active allocations; **`maand build`** clears build-owned namespaces when the job is inactive; GC purges any remainder.
3. Delete hash rows and allocation rows for removed allocations.
4. Purge stale `key_value` history (keeps the latest `MaxVersionsToKeep` versions per key).

## When to run

After build marks allocations removed:

```bash
maand build
maand deploy    # stop, remove deployed job files (keep data/logs), drop removed allocation hashes
maand gc        # delete worker jobs/<job>/ trees, purge removed allocation rows, KV references, and old KV history
```

Redeploying a removed job to the same worker before GC reuses existing **`data/`** and **`logs/`** (deploy preserves them; rsync excludes those paths).
