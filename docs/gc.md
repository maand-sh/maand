# `maand gc`

**GC** removes soft-deleted catalog rows, KV references for removed allocations, and old KV history from `maand.db`. It also deletes leftover **`data/`**, **`logs/`**, and **`bin/`** on workers when removed allocation rows are still in the database (for example if **`maand deploy`** has not run yet).

Run **`maand build`** after removing workers or jobs in the workspace so allocations are marked `removed = 1`. **`maand deploy`** stops removed allocations, removes job trees from workers, and drops the full **`/opt/worker/<bucket_id>/`** tree for hosts removed from **`workers.json`**.

## CLI

```bash
maand gc [--retain-days N]
```

| Flag | Description |
|------|-------------|
| `--retain-days` | Days to retain deleted KV versions before purge (default `0`). |

## What it does

1. For each allocation still marked `removed = 1`: SSH to the worker and `rm -rf` `jobs/<job>/data`, `logs`, and `bin` under `/opt/worker/<bucket_id>/` (no-op if deploy already removed the job tree). Workers no longer in **`workers.json`** are assumed dead when unreachable.
2. Purge KV namespaces for removed allocations (`maand/job/<job>/worker/<ip>`, and `maand/worker/<ip>` / tags when the worker is off-catalog).
3. Delete hash rows and allocation rows for removed allocations.
4. Purge stale `key_value` history (keeps the latest `MaxVersionsToKeep` versions per key).

## When to run

After build marks allocations removed:

```bash
maand build
maand deploy    # stop, remove job/bucket files on workers, drop removed allocation hashes
maand gc        # purge removed allocation rows, KV references, and old KV history
```

If you always deploy after build, GC mainly clears database rows and KV history; worker cleanup is handled by deploy.
