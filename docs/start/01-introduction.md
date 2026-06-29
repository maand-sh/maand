# 1. Introduction

Maand is an **agentless orchestrator** for a fixed pool of Linux workers. You run a single CLI on a **control machine** (laptop, bastion, or CI runner). That machine holds the **bucket** — project state on disk. Workers are ordinary hosts you reach with **SSH** and **rsync**. Nothing maand-specific is installed on workers until you **deploy** job files there.

If you have operated a small cluster with shell scripts, Ansible, and Makefiles, maand formalizes that pattern: a catalog of workers and jobs, ordered deploys, rolling restarts, and hooks — without running Kubernetes or a Nomad control plane.

---

## What runs where

```text
┌────────────── CLI host (you run `maand` here) ──────────────┐
│  workspace/     ← you edit (git)                            │
│  data/maand.db  ← catalog (maand build)                     │
│  secrets/       ← SSH key, CA, KV encryption                │
│  logs/          ← maand's own command logs                  │
└───────────────────────────┬─────────────────────────────────┘
                            │  deploy: rsync + ssh
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   Worker 10.0.0.1     Worker 10.0.0.2     …
   /opt/worker/<bucket_id>/jobs/<job>/...
```

| Location | What happens there |
|----------|-------------------|
| **CLI host** | `maand build`, `maand deploy`, job-command scripts (Python/Bun), SQLite catalog |
| **Workers** | Your app files, Makefile targets (`start`/`stop`/…), `data/` and `logs/` at runtime |

Maand does **not** replace your process supervisor or container runtime. You express lifecycle in the job **Makefile** (often calling `docker compose` or `systemctl`). Maand calls those targets during deploy and via `maand job`.

---

## Core vocabulary (preview)

| Term | One-line meaning |
|------|------------------|
| **Bucket** | One maand project directory (`maand init`) |
| **Worker** | A cluster node (SSH target) in `workers.json` |
| **Job** | A deployable unit under `workspace/jobs/<name>/` |
| **Allocation** | One job running on one worker (auto-created at build) |

Full detail: [concepts.md](./concepts.md). Next chapter maps these to tools you may already know: [02-from-your-background.md](./02-from-your-background.md).

---

## Typical workflow

```text
edit workspace  →  maand build  →  maand deploy  →  operate / debug  →  gc (after removals)
```

| Step | Touches workers? | Purpose |
|------|------------------|---------|
| **build** | No | Sync workspace → `maand.db`, validate placement, generate TLS, run `post_build` hooks |
| **deploy** | Yes | Rsync job trees, run `make start` / `restart` / `reload`, deploy hooks |
| **health_check** | Yes (probes) | Verify jobs are healthy after deploy or manually |
| **gc** | Yes (cleanup) | Purge removed allocations and worker data after you dropped jobs/workers from workspace |

Common inspect commands (any time after build):

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
```

---

## What maand is good at

- **Small to medium fleets** (often 10–100 workers) with a stable host list
- **Stateful or scripted services** — databases, queues, compose stacks, systemd units
- **Ordered multi-job deploys** — job B waits for job A via manifest **demands**
- **Rolling restarts** with batch size and optional health gates
- **Drain / disable** a node or allocation without deleting workspace definitions
- **Inspectable state** — dry-run deploy, `maand cat deployments`, structured logs

---

## Intentional limits

| Maand is not… | Instead… |
|---------------|----------|
| Highly available control plane | One bucket directory on one CLI host is the source of truth |
| A container scheduler | You bring Docker/systemd via Makefile |
| Dynamic bin-packing | Placement is **label matching** + resource bounds |
| A log aggregation platform | App logs live on workers; maand logs its own commands |

See [overview.md](./overview.md) for a full capability map and [comparison-orchestrators.md](./comparison-orchestrators.md) for Kubernetes/Nomad contrast.

---

## Next

[02 — From your background](./02-from-your-background.md) · or jump to [quickstart](./quickstart.md) if you learn by doing.
