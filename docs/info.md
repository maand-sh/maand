# `maand info`

Prints a summary of the current bucket from `maand.db`.

## CLI

```bash
maand info
```

## Output

| Field | Meaning |
|-------|---------|
| Bucket ID | Unique bucket identifier (UUID); appears in worker paths `/opt/worker/<bucket_id>/` |
| Update Sequence | Incremented on each deploy; mirrored in worker `worker.json` |
| Number of Workers | Workers in the catalog |
| Number of Jobs | Jobs in the catalog |
| Number of Allocations | Active + disabled + removed allocation rows |

Use **`maand cat workers`**, **`maand cat jobs`**, and **`maand cat allocations`** for detailed tables. For rollout state per allocation, use **`maand cat deployments`** — see [deploy-debugging.md](./deploy-debugging.md).
