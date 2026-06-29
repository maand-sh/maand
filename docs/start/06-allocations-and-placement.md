# 6. Allocations and placement

An **allocation** is the binding **(job × worker)**: “run job `api` on `10.0.0.1`.”

---

## How allocations are created

During **`maand build`**, maand matches every job to every worker:

1. Collect labels on the worker (always includes `worker`).
2. Read job **selectors** from `manifest.json`. If omitted, the **job name** is the only selector.
3. If **every** selector label exists on the worker, create or update an allocation row.

```text
Worker labels:     [worker, web, prod]
Job selectors:     [worker, web]     → match → allocation (api @ 10.0.0.1)
Job selectors:     [worker, gpu]     → no match
```

Dedicated nodes: a job named `prometheus` on a worker labeled `prometheus` can omit selectors — the job name becomes the selector.

---

## Allocation identity

| Field | Meaning |
|-------|---------|
| `alloc_id` | Stable UUID (derived from job + worker IP) |
| `worker_ip` / `job` | The pair |
| `disabled` | `1` = drained (see below) |
| `removed` | `1` = soft-deleted (worker or job left workspace) |
| `zone` | Worker tag `zone` from `workers.json` (empty when unset) |

Inspect:

```bash
maand cat allocations
maand cat allocations --jobs api --workers 10.0.0.1
maand cat deployments    # hash + version state per allocation
```

The **`zone`** column comes from each worker's **`tags.zone`** entry in `workers.json`.

---

## Active vs disabled vs removed

| State | `removed` | `disabled` | Deploy lifecycle |
|-------|-----------|------------|------------------|
| **Active** | 0 | 0 | rsync + start/restart/reload |
| **Disabled** | 0 | 1 | stop; no start/restart; still built & staged |
| **Removed** | 1 | * | stop + remove deployed files; data/logs kept until GC |

**Disabled** is for maintenance drain without deleting definitions. Edit **`workspace/disabled.json`**:

```json
{
  "jobs": {
    "api": { "allocations": ["10.0.0.2"] }
  },
  "workers": ["10.0.0.3"]
}
```

| Intent | `disabled.json` |
|--------|-----------------|
| Drain one allocation | `"api": { "allocations": ["10.0.0.2"] }` |
| Drain all instances of a job | `"api": {}` |
| Drain every job on a host | `"workers": ["10.0.0.3"]` |

Then **`maand build`** and **`maand deploy`**.

Guide: [disable-and-drain.md](../guides/disable-and-drain.md).

---

## Resource validation

At build, maand checks:

- Sum of job memory/CPU reservations (from `bucket.jobs*.conf`) against worker capacity
- Port collisions between jobs
- Manifest min/max vs assigned reservations
- **`min_allocations_count`** vs non-removed allocation count per job

Failures stop **build** before any deploy. Details: [resources-and-placement.md](../reference/resources-and-placement.md).

---

## One job, many workers; one worker, many jobs

```text
        api@10.0.0.1    api@10.0.0.2
              \            /
               job "api"
              /            \
        cache@10.0.0.1   cache@10.0.0.2

Worker 10.0.0.1 might host: api, cache, node_exporter, …
```

Each cell is one allocation with its own `alloc_id`, version tracking, and KV subtree.

---

## Next

[07 — Build](./07-build.md) — turn workspace into the catalog.
