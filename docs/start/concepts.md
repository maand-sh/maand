# Core concepts

Maand is an **agentless** workload orchestrator. You run the **`maand` CLI** on a **host machine** (laptop, CI runner, or bastion). That host holds the **bucket** — local files and SQLite state. Workers are ordinary Linux hosts reached over **SSH**; nothing is installed on them except what you deploy (job files, `runner.py`, and runtime directories).

```text
┌──────────────────── maand CLI host ────────────────────┐
│  bucket/                                               │
│    maand.db + KV    workspace/    secrets/    tmp/     │
│         │                │                             │
│         │    build reads │                             │
│         ▼                ▼                             │
│    catalog (workers, jobs, allocations)                │
└────────────────────────┬───────────────────────────────┘
                         │ ssh + rsync (deploy, job control)
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    Worker A         Worker B         Worker C
    /opt/worker/<bucket_id>/
```

Official site: [maand.sh/latest](https://maand.sh/latest)

---

## Bucket

A **bucket** is one maand project directory. Run all maand commands from that directory (or with `bucket.Location` set to it in tests).

| Path | Role |
|------|------|
| `maand.conf` | SSH user/key, sudo, cert TTL, `job_config_selector`, `log_format` |
| `data/maand.db` | SQLite catalog: workers, jobs, allocations, hashes, KV history |
| `workspace/` | Source of truth you edit: `workers.json`, `jobs/<name>/`, optional `disabled.json` |
| `secrets/` | CA, worker SSH private key, KV encryption key |
| `tmp/` | Staging for deploy and job-command workspaces |
| `logs/` | Structured command logs from deploy, rsync, SSH, job commands (`logs/runs/<run_id>/` per invocation) |

Each bucket has a stable **`bucket_id`** (UUID) and an **`update_seq`** incremented on every deploy. Workers store both in `worker.json` so manual job control can detect drift.

**Build** reads `workspace/` → updates `maand.db` and KV. **Deploy** reads the DB → rsyncs to workers. Workers are never the source of truth for catalog data.

---

## Worker

A **worker** is a cluster node — an SSH target identified by **IP or hostname**.

### Definition

Workers are declared in **`workspace/workers.json`**:

```json
[
  {
    "host": "10.0.0.1",
    "labels": ["worker", "gpu"],
    "memory": "8192 mb",
    "cpu": "4000 mhz",
    "tags": { "zone": "a", "rack": "1" }
  }
]
```

| Field | Meaning |
|-------|---------|
| `host` | SSH address (unique) |
| `labels` | Used for job placement; label **`worker`** is added automatically |
| `memory` / `cpu` | Capacity for resource validation when jobs declare limits |
| `tags` | Arbitrary metadata → KV namespace `maand/worker/<ip>/tags/<key>` |
| `position` | Order in the array (assigned on read) |

### On the worker after deploy

```text
/opt/worker/<bucket_id>/
├── worker.json          # bucket_id, worker_id, update_seq, labels
├── jobs.json            # list of jobs on this worker (+ disabled flag)
├── bin/runner.py        # runs Makefile targets via SSH job control
└── jobs/<job>/          # staged job tree (Makefile, configs, data/, logs/, bin/)
```

### Worker lifecycle

| Event | What happens |
|-------|----------------|
| Add host to `workers.json` + **build** | New row in `worker` table |
| **`maand worker_facts`** | Probe host over SSH; update **`memory`** / **`cpu`** in `workers.json` (then **`maand build`** to sync catalog) |
| **Deploy** | Creates `/opt/worker/<bucket_id>/`, syncs jobs |
| Remove host from `workers.json` + **build** | Allocations marked **`removed`**; worker row dropped from catalog |
| **Deploy** after removal | Stop job; remove deployed files; keep `data/` and `logs/`; clear allocation hash on deploy (redeploy starts fresh, reuses data/logs) |
| **GC** | Delete worker `data/`/`logs/`/`bin/`; purge removed allocation rows and KV |

Workers do not run a maand agent. Deploy and `maand job` invoke **`runner.py`** over SSH; **`maand worker_facts`** probes capacity into `workers.json`; **`maand run_command`** runs arbitrary shell commands over SSH.

---

## Job

A **job** is a deployable unit — a directory under **`workspace/jobs/<name>/`**.

### Required layout

```text
workspace/jobs/api/
├── manifest.json       # version, selectors, resources, commands, certs
├── Makefile            # start / stop / restart (default deploy lifecycle)
├── _modules/           # optional: command_<name>.py | .ts | .js
├── config.tpl          # optional: Go templates → rendered at deploy
└── …                   # other files copied into maand.db on build
```

### Manifest highlights

| Field | Purpose |
|-------|---------|
| `version` | Semver-like release id; required when the job participates in the [dependency graph](../reference/deployment-sequence.md); drives deploy **`new_version`** per allocation |
| `selectors` | Worker **labels** for placement; when omitted, the **job name** is used |
| `resources.memory` / `cpu` | **Min/max bounds** in the manifest; **actual** memory/CPU for the current environment from `bucket.jobs.conf` or `bucket.jobs.<env>.conf` (selected by `job_config_selector` in `maand.conf`) — [resources-and-placement.md](../reference/resources-and-placement.md) |
| `resources.ports` | Named ports: `{}` (maand assigns from `bucket.conf` pool) or an integer (fixed in manifest) |
| `update_parallel_count` | Rolling batch size for **`restart`** / **`reload`** during deploy |
| `restart_policy` | `always` / `reload` / `never` — how upgrades apply after rsync ([deploy](../reference/cli/deploy.md#applying-changes-on-workers)) |
| `restart_globs` | Optional; with `reload`, paths that force **`restart`** when changed |
| `deploy_parallel_count` | Rolling batch size for **`start`** on first deploy (0 = all at once) |
| `commands` | Named hooks (`command_*`) with `executed_on` events |
| `health_check` | Optional built-in probes (tcp/http/ssh) and/or a `health_check` command (probes first) |
| `certs` | TLS definitions → generated at build, deployed under `jobs/<job>/certs/` — [certs.md](../reference/certs.md) |

Manifest reference: [manifest.md](../reference/manifest.md). Configuration: [configuration.md](../reference/configuration.md). Command scripts: [cli/job-command.md](../reference/cli/job-command.md).

### Job lifecycle on workers

Each deploy wave **rsyncs** the job tree, then optionally runs Makefile targets on the worker:

1. **New allocation** → **`make start`**
2. **Upgrade** → depends on **`restart_policy`**:
   - **`always`** → **`make restart`**
   - **`reload`** → **`make reload`**, or **`make restart`** when a changed file matches **`restart_globs`**
   - **`never`** → no lifecycle (files only)

The Makefile receives **`CURRENT_VERSION`** (running) and **`NEW_VERSION`** (target). Custom rollouts use a **`job_control`** command instead of the default targets.

Runtime state lives in **`data/`**, **`logs/`**, and **`bin/`** on the worker — not in the workspace (build rejects those dirs in git).

---

## Allocation

An **allocation** is the binding **(job × worker)**: “run job *api* on worker *10.0.0.1*.”

Maand creates allocations automatically during **build** by **label matching**:

1. For each worker, collect its labels (including `worker`).
2. For each job, use manifest `selectors` when set; otherwise use the **job name** as the selector.
3. Require **every** selector to appear on the worker.
4. Insert or update a row in **`allocations`** for each match.

```text
workers.json              jobs/api/manifest.json
  labels: [worker,         selectors: [worker, prod]
           prod]                   │
       │                           │  selectors: [worker, prod]
       └──── all match? ───────────┘
                 │
                 ▼
        allocation (api @ 10.0.0.1)
        alloc_id = hash("api|10.0.0.1")
```

Dedicated jobs can omit manifest selectors when the worker carries the job name (for example job **`prometheus`** on a worker labeled **`prometheus`**).

### Allocation fields

| Column | Meaning |
|--------|---------|
| `alloc_id` | Stable UUID derived from job + worker IP |
| `worker_ip` / `job` | The pair |
| `disabled` | `1` when excluded via `disabled.json` or resource rules |
| `removed` | `1` when worker or job left the workspace (soft delete until deploy/GC) |
| `deployment_seq` | Wave order during deploy (from command **demands**) |

Each allocation tracks catalog **`current_version`** (last promoted, in **`hash`**) and **`new_version`** (build target, in **`allocations`**). Per-allocation KV stores a single **`version`** key (target). Templates and job commands expose running vs target via **`.CurrentVersion`** / **`.NewVersion`** and **`CURRENT_VERSION`** / **`NEW_VERSION`** — see [deploy.md](../reference/cli/deploy.md#allocation-version-tracking).

### Active vs inactive

An allocation is **active** when `removed = 0` and `disabled = 0`. A **disabled** allocation (`removed = 0`, `disabled = 1`) still gets build KV (certs, per-allocation metadata, deploy staging). Deploy **never starts** disabled allocations (no start/restart/reload/rsync). Content and version changes are still staged and promoted; after re-enable, deploy **starts** the allocation.

**KV nuance:** `maand/job/<job>/workers`, `maand/job/<job>/deploy_order`, and `maand/worker/<ip>/jobs` list **active** allocations only. Per-allocation keys such as **`peer_workers`** use **non-removed** peers (disabled peers may still appear). See [KV namespaces](../reference/kv/namespaces.md).

Only **active** allocations receive:

- Deploy rsync and lifecycle (start/restart/reload) plus rollout hooks that run on workers
- `maand job_command`, `maand health_check`
- Default targets for `maand job start|stop|restart|run --target reload`

Inspect allocations:

```bash
maand cat allocations
maand cat allocations --jobs api --workers 10.0.0.1
```

### Disabling without removing

**`workspace/disabled.json`** marks allocations disabled without deleting workspace files:

```json
{
  "jobs": {
    "api": { "allocations": ["10.0.0.2"] }
  },
  "workers": ["10.0.0.3"]
}
```

- Disable all instances of a job: `"api": {}`
- Disable one worker for a job: `"allocations": ["10.0.0.2"]`
- Disable every job on a worker: `"workers": ["10.0.0.3"]`

Run **`maand build`** after editing `disabled.json`.

Full how-to (disable one allocation, entire job, entire worker, re-enable): [disable and drain](../guides/disable-and-drain.md).

---

## How the three relate

| Concept | Question it answers | Example |
|---------|---------------------|---------|
| **Worker** | *Where* can work run? | `10.0.0.1` with labels `worker`, `gpu` |
| **Job** | *What* workload? | `api` — manifest, Makefile, files |
| **Allocation** | *Which job on which worker?* | `api` on `10.0.0.1`, `alloc_id=…` |

One job typically has **many** allocations (one per matching worker). One worker typically hosts **many** jobs (one allocation per job).

---

## Deployment sequence

Jobs that depend on each other (via **`demands`** in manifest commands) get a **`deployment_seq`** during build. **Deploy** processes sequence **0**, then **1**, and so on — so depended-on jobs reach workers before dependents.

Full reference: [deployment-sequence.md](../reference/deployment-sequence.md).

Details in [cli/build.md](../reference/cli/build.md) and [cli/deploy.md](../reference/cli/deploy.md).

---

## Job commands vs Makefile vs run_command

| Mechanism | Runs on | Trigger | Use case |
|-----------|---------|---------|----------|
| **Makefile** (`start`/`stop`/`restart`/`reload`) | Worker | Deploy, `maand job` | Process lifecycle |
| **Job commands** (`command_*`) | CLI host | build/deploy/CLI/health_check | Migrations, KV, hooks |
| **`maand run_command`** | Worker (raw shell) | Manual | Ops, debugging |
| **`maand worker_facts`** | Worker (probe) | Manual | Fill `memory` / `cpu` in `workers.json` |

Job commands talk to maand’s **runtime HTTP API** and **KV store** on the CLI host. See [cli/job-command.md](../reference/cli/job-command.md) and [job-command-api.md](../reference/job-command-api.md).

---

## Typical state flow

```text
edit workspace/workers.json, workspace/jobs/*
        │
        ▼
   maand build          ← catalog + KV + certs; no SSH to workers
        │
        ▼
   maand deploy         ← rsync + lifecycle (start/restart/reload) + hooks
        │
        ├── maand health_check
        ├── maand job restart <job>
        ├── maand jobcommand <cmd> [job]
        ├── maand worker_facts   # optional: refresh worker capacity
        ├── maand run_command "…"
        └── maand gc     ← after removals
```

---

## Further reading

- [overview.md](./overview.md) — capabilities and limits
- [quickstart.md](./quickstart.md) — hands-on first deploy
- [reference/README.md](../reference/README.md) — configuration, manifest, CLI, KV, logging
- [../README.md](../README.md) — full doc index
