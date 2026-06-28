# Job manifest reference

Canonical schema for **`workspace/jobs/<name>/manifest.json`**. For concepts (worker, job, allocation), see [start/concepts](../start/concepts.md). For dependency ordering, see [deployment-sequence.md](./deployment-sequence.md).

---

## Job layout

```text
workspace/jobs/database/
├── manifest.json       # required
├── Makefile            # required unless deploy uses only job_control commands
├── Makefile.tpl        # alternative to Makefile (rendered at deploy)
├── vars.toml           # optional application config → vars/job/<job>
├── _modules/           # optional: command_<name>.py | .ts | .js
├── *.tpl               # optional Go templates (rendered at deploy)
└── _prometheus/        # optional metrics — see [guides/prometheus](../guides/prometheus.md)
```

Scaffold:

```bash
maand job create myservice --selectors worker
```

Inspect built catalog:

```bash
maand cat jobs              # includes deployment_seq
maand cat job_commands
```

---

## Top-level fields

| Field | Purpose |
|-------|---------|
| `version` | Semver-like release id; see [Version](#version) |
| `selectors` | Worker **labels** required for placement (all must match). When omitted, the **job name** is used — see [Placement selectors](#placement-selectors). |
| `update_parallel_count` | Rolling **restart** batch size during deploy (default **1**) |
| `deploy_parallel_count` | Rolling **start** batch size on first deploy (**0** = all at once) |
| `restart_policy` | How deploy applies **updated** allocations after rsync: `always`, `reload`, or `never` (default `always`) |
| `restart_globs` | With `reload` only — globs; matching changed paths trigger `restart` instead of `reload` |
| `resources` | Memory, CPU, ports — [resources-and-placement.md](resources-and-placement.md) |
| `commands` | Named hooks (`command_*`) — [cli/job-command.md](./cli/job-command.md) |
| `health_check` | Built-in probes (tcp/http/ssh) and/or a `health_check` command (probes run first) |
| `certs` | TLS definitions → KV per allocation — [certs.md](certs.md) |

Example:

```json
{
  "version": "2.1.0",
  "selectors": ["worker"],
  "update_parallel_count": 2,
  "deploy_parallel_count": 1,
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

Port `{}` assigns from the `bucket.conf` pool; an integer fixes the port in the manifest.

---

## Memory and CPU (`resources`)

Use **`resources.memory`** and **`resources.cpu`** in the manifest to declare **min** and **max** bounds for the job. These limits travel with the job in git and define what reservations are allowed on any bucket.

```json
"resources": {
  "memory": { "min": "256 mb", "max": "2 gb" },
  "cpu": { "min": "500 mhz", "max": "2000 mhz" }
}
```

| Field | Meaning |
|-------|---------|
| `min` | Lower bound for reservations (omit or `0` = no lower bound) |
| `max` | Upper bound for reservations (if `min` is set and `max` omitted, `max` defaults to `min`) |

Build fails when **`min` > `max`**.

### Actual reservation (not in the manifest)

The manifest does **not** set how much memory or CPU the job uses **today**. That comes from **`workspace/bucket.jobs.conf`** (or an environment-specific file — see below). Each job section sets the **current** reservation:

```toml
# workspace/bucket.jobs.prod.conf
[api]
memory = "1 gb"
cpu = "1500 mhz"
```

The value must satisfy **`manifest min ≤ reservation ≤ manifest max`**. Build stores it as **`current_memory_mb`** / **`current_cpu_mhz`** and sums it per worker for capacity checks against **`workers.json`**.

### Choosing an environment

Set **`job_config_selector`** in **`maand.conf`** to pick which override file build reads:

| `job_config_selector` | File |
|-----------------------|------|
| `""` | `workspace/bucket.jobs.conf` |
| `"prod"` | `workspace/bucket.jobs.prod.conf` |
| `"staging"` | `workspace/bucket.jobs.staging.conf` |

Use the same manifest bounds in every environment; tune **`memory`** / **`cpu`** per file without changing `manifest.json`.

KV keys **`maand/job/<job>`** expose `min_memory_mb`, `max_memory_mb`, `memory`, `min_cpu_mhz`, `max_cpu_mhz`, and `cpu` for templates and commands.

Details: [resources-and-placement.md](resources-and-placement.md) · [configuration.md](configuration.md#job-memory-and-cpu-manifest-bounds-vs-bucket-overrides).

---

## Placement selectors

If `selectors` is set in the manifest, **only** those labels are used for placement. If `selectors` is omitted or empty, the **job name** is the sole selector. A worker must have every required selector as a label (plus the automatic `worker` label from `workers.json`).

| Manifest | Effective selectors | Worker labels needed |
|----------|---------------------|----------------------|
| *(empty)* | `prometheus` | `prometheus`, `worker` |
| `"selectors": ["worker"]` | `worker` | `worker` |
| `"selectors": ["worker", "prod"]` | `worker`, `prod` | `worker`, `prod` |

Dedicated jobs (for example **`prometheus`**) can omit `selectors` when the target worker is labeled with the job name:

```json
{}
```

Label that worker in **`workers.json`**:

```json
{ "host": "10.0.0.5", "labels": ["prometheus"], "memory": "4 gb", "cpu": "2 ghz" }
```

Shared pool jobs use `"selectors": ["worker"]`. Environment-specific jobs add labels such as `"prod"` / `"staging"` — see [resources-and-placement.md](resources-and-placement.md).

---

## Version

**`version`** identifies the job release for KV, templates, dependency checks, and deploy upgrade tracking.

### Build target vs running version

| Layer | When | Where | Meaning |
|-------|------|-------|---------|
| **Target** | `maand build` | `job.version`, `maand/job/<job>/version` | From manifest (**`0.0.0`** when omitted) |
| **Per allocation** | build / deploy | `hash.current_version`, `allocations.new_version` | Running vs build target (catalog) |
| **Per-allocation KV** | deploy plan | `maand/job/<job>/worker/<ip>/version` | Target version for templates and `maand cat kv` |
| **Templates (`.tpl`)** | deploy | `{{ .CurrentVersion }}`, `{{ .NewVersion }}` | Running vs target on allocation context |

After deploy **promote**, catalog **`current_version`** becomes **`new_version`**. Per-allocation KV **`version`** holds the build target. First deploy starts at **`current_version = 0.0.0`**.

Worker **`make start`/`restart`** and job commands receive **`CURRENT_VERSION`** and **`NEW_VERSION`**. Details: [cli/deploy.md](./cli/deploy.md#allocation-version-tracking).

### Format

| Input | Parsed as |
|-------|-----------|
| `"1.0.0"` | 1.0.0 |
| `"v2.1"` | 2.1.0 |
| `"3"` | 3.0.0 |
| `"2.0.0-rc1"` | 2.0.0-rc1 |

Invalid: empty, `unknown`, `1.2.3.4`, non-numeric segments.

### When required

| Job type | `version` in manifest |
|----------|----------------------|
| Standalone (no demands) | Optional (stored as `0.0.0`) |
| Has `demands` on a command | **Required** |
| Upstream job demanded by another | **Required** |

Build validates **`demands.config.min_version`** / **`max_version`** against upstream `version`. See [deployment-sequence.md](./deployment-sequence.md).

### Inspect

```bash
maand cat kv get maand/job/database version
maand cat kv get maand/job/api/worker/10.0.0.1 version
maand cat certs --jobs api
```

---

## Commands block

Commands live in **`manifest.json` → `commands`** and as scripts under **`_modules/`**.

| Rule | Detail |
|------|--------|
| Name | Must start with **`command_`** |
| Script | Exactly one of `command_<name>.py`, `.ts`, or `.js` |
| `executed_on` | One or more allowed events (see [cli/job-command.md](./cli/job-command.md#command-events-executed_on)) |
| `demands` | Optional upstream job/command dependency — [deployment-sequence.md](./deployment-sequence.md) |

Allowed **`executed_on`** values:

`post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, `cli`, `after_allocation_started`, `after_allocation_stopped`

Each `(command, executed_on)` pair becomes one row in **`job_commands`**.

Empty dependency (default):

```json
"demands": { "job": "", "command": "", "config": {} }
```

---

## Deploy rollout

These fields control **how** deploy applies an upgrade after files are rsynced. They do not affect **build** or placement. Full behavior: [cli/deploy.md](./cli/deploy.md#applying-changes-on-workers).

### `restart_policy`

| Value | Meaning |
|-------|---------|
| **`always`** (default) | Run **`make restart`** on every content or version upgrade |
| **`reload`** | Run **`make reload`** when possible; pair with **`restart_globs`** for paths that need a full restart |
| **`never`** | Rsync and promote only; no Makefile lifecycle on upgrade |

New allocations always run **`make start`**, regardless of policy.

### `restart_globs`

Optional string array. Valid **only** when **`restart_policy`** is **`reload`**. Each entry is a glob relative to the job directory (`Makefile`, `bin/**`, `docker-compose.yml`, etc.).

Maand diffs per-file content hashes between the staged tree and the last promoted tree. If any **changed** path matches a glob, deploy runs **`restart`** instead of **`reload`** on that allocation.

Example:

```json
{
  "restart_policy": "reload",
  "restart_globs": ["docker-compose.yml", "Dockerfile", "bin/**"]
}
```

Requires a **`reload:`** target in the Makefile (and **`restart:`** for glob-triggered full restarts).

---

## Rollout fields (summary)

| Field / KV | Purpose | Guide |
|------------|---------|-------|
| `deploy_parallel_count` | Batch size for **first deploy** starts | [guides/rolling-deploy](../guides/rolling-deploy.md) |
| `update_parallel_count` | Batch size for **restart** / **reload** upgrades | [guides/rolling-deploy](../guides/rolling-deploy.md) |
| `restart_policy` | `always` / `reload` / `never` on updated allocations | [cli/deploy.md](./cli/deploy.md#applying-changes-on-workers) |
| `restart_globs` | Critical paths when policy is `reload` | [cli/deploy.md](./cli/deploy.md#applying-changes-on-workers) |
| `deploy_order` KV | Worker order within batches (build-synced; override via **`put_deploy_order`** in `pre_deploy` or `cli`) | [kv/namespaces.md](./kv/namespaces.md) |

---

## Related

- [deployment-sequence.md](./deployment-sequence.md) — demands, `deployment_seq`, deploy waves
- [cli/job-command.md](./cli/job-command.md) — hook events and patterns
- [cli/build.md](./cli/build.md) · [cli/deploy.md](./cli/deploy.md)
- [resources-and-placement.md](resources-and-placement.md)
- [guides/job-commands-tutorial.md](../guides/job-commands-tutorial.md)
