# Jobs, dependencies, and deployment sequence

This reference describes how **jobs** are defined in maand, how **command demands** express dependencies between jobs, and how **`deployment_seq`** orders deploy and `post_build` waves.

Related: [concepts.md](./concepts.md) · [configuration.md](./configuration.md) · [job-command.md](./job-command.md) · [kv.md](./kv.md) · [templates.md](./templates.md) · [build.md](./build.md) · [deploy.md](./deploy.md)

---

## Job definition

A **job** is a directory under **`workspace/jobs/<name>/`**. Build copies it into the catalog (`job`, `job_files`, `job_commands`, …) and matches it to workers via **selectors** (see [concepts.md](./concepts.md#job)).

### Minimum layout

```text
workspace/jobs/database/
├── manifest.json       # required
├── Makefile            # required unless deploy uses only job_control commands
└── _modules/           # optional: command_<name>.py | .ts | .js
```

### Manifest fields (job-level)

| Field | Purpose |
|-------|---------|
| `version` | Semver-like string (`1.2.0`, optional `-rc1`); required for jobs in the dependency graph |
| `selectors` | Worker labels required for placement (all must match) |
| `update_parallel_count` | Rolling restart batch size **within one deploy wave** (default 1) |
| `resources` | Memory, CPU, ports (validated at build) |
| `commands` | Named hooks — see below |
| `certs` | TLS definitions → KV per allocation |

Example:

```json
{
  "version": "2.1.0",
  "selectors": ["worker"],
  "update_parallel_count": 2,
  "resources": {
    "memory": { "min": "256 mb", "max": "1 gb" },
    "cpu": { "min": "200 mhz", "max": "1000 mhz" },
    "ports": { "database_port": 5432, "http_port": {} }
  },
  "commands": {
    "command_schema": {
      "executed_on": ["post_build", "pre_deploy"],
      "demands": { "job": "", "command": "", "config": {} }
    }
  }
}
```

Scaffold a new job:

```bash
maand job create myservice --selectors worker
```

Inspect built jobs (includes **`deployment_seq`**):

```bash
maand cat jobs
```

Jobs are sorted by `deployment_seq`, then name.

---

## Job version

**`version`** in `manifest.json` identifies the job release for KV, templates, dependency checks, and **deploy upgrade tracking**.

### Build target vs running version

| Layer | When | Where | Meaning |
|-------|------|-------|---------|
| **Target** | `maand build` | `job.version`, `maand/job/<job>/version` | Version from manifest (normalized to **`0.0.0`** when omitted) |
| **Running / target per allocation** | `maand build` / `maand deploy` | `hash.current_version`, `allocations.new_version` | Running vs build target for that worker allocation |
| **Per-allocation KV** | deploy plan | `maand/job/<job>/worker/<ip>/current_version`, `new_version` | Same values for templates and `maand cat kv` |

After a successful deploy **promote**, `current_version` becomes `new_version` for that allocation. First deploy starts with **`current_version = 0.0.0`**.

Worker **`make restart`** and job command scripts receive **`CURRENT_VERSION`** and **`NEW_VERSION`**. Details: [deploy.md](./deploy.md#allocation-version-tracking).

### Format

| Input | Parsed as |
|-------|-----------|
| `"1.0.0"` | 1.0.0 |
| `"v2.1"` | 2.1.0 |
| `"3"` | 3.0.0 |
| `"2.0.0-rc1"` | 2.0.0-rc1 (prerelease &lt; release) |

Invalid: empty, `unknown`, `1.2.3.4`, non-numeric segments.

### When required

```text
standalone job          → version optional
job with demands        → version required
job demanded by another → version required
```

If **`database`** is at `1.0.0` and **`api`** declares `"min_version": "2.0.0"`, **`maand build`** fails until **`database`** is bumped or the constraint is relaxed.

### Inspect

```bash
maand cat jobs
maand cat kv get maand/job/database version
maand cat kv get maand/job/api/worker/10.0.0.1 current_version
maand cat kv get maand/job/api/worker/10.0.0.1 new_version
```

---

## Job commands

Commands live in **`manifest.json`** → **`commands`** and as scripts under **`_modules/`**.

| Rule | Detail |
|------|--------|
| Name | Must start with **`command_`** |
| Script | Exactly one of `command_<name>.py`, `.ts`, or `.js` |
| `executed_on` | One or more of: `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, `cli` (a job cannot also define manifest **`health_check`** probes — see [health-check.md](./health-check.md)) |
| `demands` | Optional dependency on another job’s command (see below) |

Each `(command, executed_on)` pair becomes one row in **`job_commands`**. Inspect:

```bash
maand cat job_commands
```

| Column | Meaning |
|--------|---------|
| `job` | Job that owns the command |
| `command_name` | e.g. `command_schema` |
| `executed_on` | Event that may run this command |
| `demand_job` | Job that must deploy/run first (empty = no dependency) |
| `demand_command` | Command name on `demand_job` |
| `demand_config` | JSON passed to dependents via runtime **`GET /demands`** |

Empty dependency (default):

```json
"demands": {
  "job": "",
  "command": "",
  "config": {}
}
```

---

## Command demands (job dependencies)

A **demand** says: before this command runs in the dependency graph, the named job must appear **earlier** in the deployment order. Demands are declared per command, but **`deployment_seq` is computed per job** (see next section).

```json
{
  "commands": {
    "command_migrate": {
      "executed_on": ["pre_deploy"],
      "demands": {
        "job": "database",
        "command": "command_schema",
        "config": {
          "min_version": "2.0.0"
        }
      }
    }
  }
}
```

| Field | Role |
|-------|------|
| `demands.job` | Upstream job name |
| `demands.command` | Upstream command name (on that job) |
| `demands.config` | JSON for dependents; **`min_version`** / **`max_version`** validated at build; other keys for runtime **`maand.demands()`** |

### What demands validate at build

After all jobs are synced, **`maand build`** runs **`ValidateJobCommandDemands`**:

| Check | Error |
|-------|--------|
| `demands.job` and `demands.command` both set or both empty | `ErrInvalidJobCommandDemand` |
| `demands.job` exists in workspace | `ErrInvalidJobCommandDemand` |
| `demands.command` declared on upstream job | `ErrInvalidJobCommandDemand` |
| Dependency participants declare **`version`** | `ErrInvalidJobVersion` |
| `version` is parseable semver (`major.minor.patch`, optional `-prerelease`) | `ErrInvalidJobVersion` |
| `demands.config.min_version` / `max_version` vs upstream `version` | `ErrJobCommandDemandVersionMismatch` |

Version constraint example:

```json
"demands": {
  "job": "database",
  "command": "command_schema",
  "config": {
    "min_version": "2.0.0",
    "max_version": "3.0.0"
  }
}
```

`min_version` and `max_version` accept strings (`"2.1.0"`) or integers (`2` → `2.0.0`).

Jobs with **no** dependency involvement may omit `version` (stored as **`0.0.0`** in job KV and allocation version fields). Jobs that **demand** another job or are **demanded** must declare an explicit non-empty `version` string in the manifest.

### What demands do **not** do at runtime

- They do **not** automatically run the upstream command when the downstream command runs.
- At runtime, scripts can still call **`GET /demands`** to see dependent commands and `demand_config`.

### Runtime: who depends on me?

While a command runs, it can query which other commands list it as a demand:

```python
import maand
for d in maand.demands():
    print(d["job"], d["command"], d["demand_config"])
```

This maps to **`GET /demands`** with headers **`X-ALLOCATION-ID`** and **`X-COMMAND-NAME`**.

---

## `deployment_seq`

**`deployment_seq`** is an integer assigned to every **job** (and copied to all its **allocations**) during **`maand build`**. Lower numbers deploy first.

### How it is computed

After **`BuildAllocations`**, **`BuildDeploymentSequence`**:

1. Builds a graph from **`job_commands`** rows where **`demand_job != ''`** (edge: dependent job → upstream `demand_job`).
2. Seeds jobs that have at least one command row with **`demand_job = ''`** at level **0**.
3. Recursively assigns level **N+1** to jobs whose commands demand a job at level **N**.
4. Sets each job’s **`deployment_seq`** to the **maximum** level reached by any of its commands (longest dependency chain).

Jobs with **no commands** keep sequence **0** (initial allocation default).

### Example chain

```text
database (seq 0)  ──demand──►  api (seq 1)  ──demand──►  frontend (seq 2)
```

Manifest sketch:

| Job | Command | demands.job | demands.command | deployment_seq |
|-----|---------|-------------|-----------------|----------------|
| `database` | `command_schema` | *(empty)* | *(empty)* | **0** |
| `api` | `command_migrate` | `database` | `command_schema` | **1** |
| `frontend` | `command_assets` | `api` | `command_migrate` | **2** |

### Multiple demands → max depth

If one job has several commands with different upstream jobs, its sequence is the **deepest** chain:

```text
        a (0)
       / \
      b (1)   \
       \       \
        c (2)   d (2)   ← d demands both a and b; seq = max(1, 2) = 2
```

(from integration tests: jobs `a`→`b`→`c`, job `d` demands both `a` and `b` → `d` gets seq **2**)

### Circular demands

Cycles fail **`maand build`**:

```text
ErrCircularJobCommandDependency: a -> b -> a
```

Fix the manifest so the demand graph is a DAG.

---

## Where `deployment_seq` is used

### `maand deploy`

Deploy loops **`deployment_seq = 0 .. max`**:

```text
for seq in 0..max:
  jobs ← all jobs with deployment_seq = seq (filtered by --jobs)
  for each job in jobs:
    pre_deploy hooks → stage → rsync → start/restart → post_deploy
  commit partial progress; continue to next seq even if one job failed
```

Within one sequence value:

- All matching jobs are eligible in the **same wave**.
- They share worker staging under `tmp/workers/<ip>/` but roll out independently.
- **`update_parallel_count`** applies per job when restarting allocations (rolling batches on each worker set). See [rolling-upgrade.md](./rolling-upgrade.md).

Deploy **never** runs a higher sequence before a lower one completes its wave processing.

Filter to specific jobs (sequence order still enforced):

```bash
maand deploy --jobs api,frontend
```

If `frontend` is seq 2 and `api` is seq 1, deploy still processes seq 0 jobs first, then `api`, then `frontend`.

### `maand build` — `post_build`

After the main build transaction commits, **`post_build`** hooks run in the **same sequence order** (seq 0 jobs first, then seq 1, …). Any hook failure **fails the build**.

### Not ordered by `deployment_seq`

| Command | Ordering |
|---------|----------|
| `maand job start/stop/restart` | Manual; no sequence |
| `maand job_command` (cli) | Runs on selected job only |
| `maand health_check` | Per job flag; no cross-job sequence |
| `maand run_command` | Unrelated to job deps |

---

## End-to-end dependency example

**Goal:** PostgreSQL schema job must deploy before API migrations.

**1. Base job** — `workspace/jobs/database/manifest.json`:

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "commands": {
    "command_schema": {
      "executed_on": ["post_build", "cli"]
    }
  }
}
```

**2. Dependent job** — `workspace/jobs/api/manifest.json`:

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
        "config": {
          "min_version": "1.0.0"
        }
      }
    }
  }
}
```

Build accepts this because **`database`** is `1.0.0` and satisfies **`min_version`**. If **`database`** were `0.9.0`, build would fail with **`ErrJobCommandDemandVersionMismatch`**.

**3. Build and inspect:**

```bash
maand build
maand cat jobs
maand cat job_commands
```

Expect `database` → `deployment_seq 0`, `api` → `deployment_seq 1`.

**4. Deploy:**

```bash
maand deploy
```

Wave 0 rolls out `database` (including `make start`). Wave 1 runs `command_migrate` (`pre_deploy`) then rolls out `api`.

---

## Inspect and debug

```bash
maand cat jobs                              # deployment_seq per job
maand cat job_commands                      # demand_job / demand_command edges
maand cat kv get maand/job/database version # upstream version
maand cat allocations                       # same seq on all allocs for a job
maand deploy --dry-run                      # hashes only; no worker changes
```

Common build errors:

| Error | Cause |
|-------|--------|
| `ErrInvalidJobCommandDemand` | Unknown demand job/command, or partial demand pair |
| `ErrInvalidJobVersion` | Bad or missing version on dependency participant |
| `ErrJobCommandDemandVersionMismatch` | Upstream version outside min/max constraint |
| `ErrCircularJobCommandDependency` | Cycle in `demands.job` graph |
| Missing seq bump | `demands.job` left empty; fix manifest |

---

## Quick reference

| Concept | Stored in | Set by |
|---------|-----------|--------|
| Job catalog | `job`, `job_files`, … | `maand build` |
| Command + demand | `job_commands` | `maand build` |
| `deployment_seq` | `allocations.deployment_seq` | `BuildDeploymentSequence` |
| Deploy wave order | — | `maand deploy` (0 → max) |
| `post_build` order | — | `maand build` (0 → max) |
| Rolling restarts | `update_parallel_count` | Per job, within each wave |

Tutorial walkthrough: [tutorials/job-commands.md](./tutorials/job-commands.md)
