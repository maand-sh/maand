# Deployment sequence and demands

How **command demands** express cross-job dependencies and how **`deployment_seq`** orders **`maand build`** (`post_build`) and **`maand deploy`** waves.

Manifest fields: [manifest.md](./manifest.md). Concepts: [start/concepts](../start/concepts.md).

---

## Command demands

A **demand** says: the named upstream job must appear **earlier** in deployment order. Demands are declared per command; **`deployment_seq` is computed per job**.

```json
{
  "commands": {
    "command_migrate": {
      "executed_on": ["pre_deploy"],
      "demands": {
        "job": "database",
        "command": "command_schema",
        "config": { "min_version": "2.0.0" }
      }
    }
  }
}
```

| Field | Role |
|-------|------|
| `demands.job` | Upstream job name |
| `demands.command` | Upstream command on that job |
| `demands.config` | JSON for runtime `GET /demands`; **`min_version`** / **`max_version`** validated at build |

Both **`demands.job`** and **`demands.command`** must be set together or both empty. Self-reference is rejected.

### Build validation

After all jobs sync, **`ValidateJobCommandDemands`** runs:

| Check | Error |
|-------|--------|
| Partial demand pair | `ErrInvalidJobCommandDemand` |
| Unknown upstream job/command | `ErrInvalidJobCommandDemand` |
| Missing `version` on dependency participants | `ErrInvalidJobVersion` |
| Upstream version outside min/max | `ErrJobCommandDemandVersionMismatch` |
| Cycle in demand graph | `ErrCircularJobCommandDependency` |

### Runtime behavior

Demands do **not** auto-run the upstream command. Scripts can call **`GET /demands`** to see downstream dependents:

```python
import maand
for d in maand.demands():
    print(d["job"], d["command"], d["demand_config"])
```

API details: [job-command-api.md](./job-command-api.md).

---

## `deployment_seq`

Integer assigned to every **job** (and copied to all **allocations**) during **`maand build`**. Lower numbers deploy first.

### Computation

After **`BuildAllocations`**, **`BuildDeploymentSequence`**:

1. Builds a graph from **`job_commands`** where **`demand_job != ''`** (edge: dependent → upstream).
2. Seeds jobs with at least one command having empty demand at level **0**.
3. Recursively assigns level **N+1** to jobs demanding a job at level **N**.
4. Sets each job’s **`deployment_seq`** to the **maximum** level among its commands.

Jobs with **no commands** stay at sequence **0**.

### Example chain

```text
database (seq 0)  ──demand──►  api (seq 1)  ──demand──►  frontend (seq 2)
```

| Job | Command | demands | deployment_seq |
|-----|---------|---------|----------------|
| `database` | `command_schema` | *(empty)* | **0** |
| `api` | `command_migrate` | `database` / `command_schema` | **1** |
| `frontend` | `command_assets` | `api` / `command_migrate` | **2** |

Multiple demands → **max depth** across chains. Cycles fail build.

---

## Where sequence is used

### `maand deploy`

```text
for seq in 0..max:
  jobs ← all jobs with deployment_seq = seq (respect --jobs filter)
  for each job needing rollout:
    pre_deploy → stage → rsync → start/restart batches → post_deploy → promote
  commit partial progress; continue even if one job failed
```

Within one sequence value, jobs roll out **independently**. Each job applies its own **`deploy_parallel_count`** (starts) and **`update_parallel_count`** (restarts). Worker order within batches uses KV **`deploy_order`**.

Deploy **never** runs a higher sequence before a lower one finishes its wave.

Rolling batch details: [guides/rolling-deploy.md](../guides/rolling-deploy.md).

### `maand build` — `post_build`

After the main transaction commits, **`post_build`** hooks run in **sequence order** (0 first). Any failure **fails the build**.

### Not ordered by `deployment_seq`

| Command | Ordering |
|---------|----------|
| `maand job start/stop/restart` | Manual |
| `maand job_command` (cli) | Selected job only |
| `maand health_check` | Per `--jobs` flag |
| `maand run_command` | Unrelated |

---

## End-to-end example

**Goal:** Database schema before API migration.

**`workspace/jobs/database/manifest.json`:**

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "commands": {
    "command_schema": { "executed_on": ["post_build", "cli"] }
  }
}
```

**`workspace/jobs/api/manifest.json`:**

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "commands": {
    "command_migrate": {
      "executed_on": ["pre_deploy", "cli"],
      "demands": {
        "job": "database",
        "command": "command_schema",
        "config": { "min_version": "1.0.0" }
      }
    }
  }
}
```

```bash
maand build
maand cat jobs          # database seq 0, api seq 1
maand deploy            # wave 0: database; wave 1: api
```

---

## Inspect and debug

```bash
maand cat jobs
maand cat job_commands
maand cat allocations
maand deploy --dry-run
```

---

## Quick reference

| Concept | Stored in | Set by |
|---------|-----------|--------|
| Command + demand | `job_commands` | `maand build` |
| `deployment_seq` | `allocations.deployment_seq` | `BuildDeploymentSequence` |
| Deploy wave order | — | `maand deploy` (0 → max) |
| `post_build` order | — | `maand build` (0 → max) |
| Rolling restarts | `update_parallel_count` | Per job, per wave |
| Rolling first deploy | `deploy_parallel_count` | Per job, per wave |
| Rollout worker order | `deploy_order` KV | Build sync; optional `pre_deploy` override |

---

## Related

- [manifest.md](./manifest.md)
- [cli/deploy.md](./cli/deploy.md)
- [cli/build.md](./cli/build.md)
- [guides/debugging-deploy.md](../guides/debugging-deploy.md)
