# 4. Workers

A **worker** is a cluster node — an SSH target identified by IP or hostname.

---

## Declare workers

Edit **`workspace/workers.json`**:

```json
[
  {
    "host": "10.0.0.1",
    "labels": ["worker", "web"],
    "memory": "8192 mb",
    "cpu": "4000 mhz",
    "tags": { "zone": "a" }
  },
  {
    "host": "10.0.0.2",
    "labels": ["worker", "web"],
    "memory": "8192 mb",
    "cpu": "4000 mhz"
  }
]
```

| Field | Purpose |
|-------|---------|
| `host` | SSH address (must be unique) |
| `labels` | Placement tags — jobs match via **selectors** (chapter 6) |
| `memory` / `cpu` | Capacity for resource validation at build |
| `tags` | Metadata → KV `maand/worker/<ip>/tags/<key>` for templates |
| `tags.zone` | Shown as **`zone`** in `maand cat workers` and `maand cat allocations` |

Maand automatically adds the label **`worker`** to every host. You typically include `"worker"` in job selectors for a shared pool.

After editing: **`maand build`** syncs the catalog.

---

## SSH access

Test from the CLI host:

```bash
ssh -i secrets/worker.key agent@10.0.0.1 echo ok
```

All deploy, `maand job`, and `maand run_command` traffic uses the same key and user from `maand.conf`.

---

## Probe capacity with `worker_facts`

If you don't know CPU/RAM, or want to refresh them:

```bash
maand worker_facts --workers 10.0.0.1,10.0.0.2
maand worker_facts --build    # probe and run build in one step
```

This SSHs to hosts, reads memory/CPU, and writes values back into **`workers.json`**. Run **`maand build`** afterward if you didn't use `--build`.

Reference: [worker-facts.md](../reference/cli/worker-facts.md).

---

## What appears on a worker after deploy

```text
/opt/worker/<bucket_id>/
├── worker.json       # bucket_id, update_seq, labels
├── jobs.json         # jobs on this host (+ disabled flags)
├── bin/runner.py     # invoked by maand job * over SSH
└── jobs/<job>/       # synced job tree
    ├── Makefile
    ├── config        # rendered from .tpl
    └── data/         # persistent runtime state (not rsync'd over)
```

`update_seq` in `worker.json` increments on each deploy. `maand job` checks it so manual control doesn't run against a stale tree.

---

## Worker lifecycle

| You do… | Maand does… |
|---------|-------------|
| Add host to `workers.json` + **build** | New `worker` row; new **allocations** for matching jobs |
| Remove host + **build** | Allocations marked **`removed`** (soft) |
| **deploy** after removal | Stop jobs, remove deployed files; keep `data/` and `logs/` |
| **`maand gc`** | Delete worker runtime dirs; purge removed allocation rows |

Workers never run a maand agent — only files you deploy and commands you invoke over SSH.

---

## Inspect

```bash
maand cat workers
maand cat kv get maand/worker 10.0.0.1   # worker-scoped KV keys
```

---

## Next

[05 — Jobs and lifecycle](./05-jobs-and-lifecycle.md) — what you deploy onto each worker.
