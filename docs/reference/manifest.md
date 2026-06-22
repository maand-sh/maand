# Job manifest reference

Canonical schema for **`workspace/jobs/<name>/manifest.json`**. For concepts (worker, job, allocation), see [start/concepts](../start/concepts.md). For dependency ordering, see [deployment-sequence.md](./deployment-sequence.md).

---

## Job layout

```text
workspace/jobs/database/
├── manifest.json       # required
├── Makefile            # required unless deploy uses only job_control commands
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
| `selectors` | Worker **labels** required for placement (all must match) |
| `update_parallel_count` | Rolling **restart** batch size during deploy (default **1**) |
| `deploy_parallel_count` | Rolling **start** batch size on first deploy (**0** = all at once) |
| `resources` | Memory, CPU, ports — [resources-and-placement.md](resources-and-placement.md) |
| `commands` | Named hooks (`command_*`) — [cli/job-command.md](./cli/job-command.md) |
| `health_check` | Built-in probes (tcp/http/ssh); mutually exclusive with `health_check` command event |
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

## Version

**`version`** identifies the job release for KV, templates, dependency checks, and deploy upgrade tracking.

### Build target vs running version

| Layer | When | Where | Meaning |
|-------|------|-------|---------|
| **Target** | `maand build` | `job.version`, `maand/job/<job>/version` | From manifest (**`0.0.0`** when omitted) |
| **Per allocation** | build / deploy | `hash.current_version`, `allocations.new_version` | Running vs build target |
| **KV** | deploy plan | `maand/job/<job>/worker/<ip>/current_version`, `new_version` | Same values for templates |

After deploy **promote**, `current_version` becomes `new_version`. First deploy starts at **`current_version = 0.0.0`**.

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
maand cat kv get maand/job/api/worker/10.0.0.1 current_version
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

## Rollout fields (summary)

| Field / KV | Purpose | Guide |
|------------|---------|-------|
| `deploy_parallel_count` | Batch size for **first deploy** starts | [guides/rolling-deploy](../guides/rolling-deploy.md) |
| `update_parallel_count` | Batch size for **restart** upgrades | [guides/rolling-deploy](../guides/rolling-deploy.md) |
| `deploy_order` KV | Worker order within batches (build-synced; override via **`put_deploy_order`** in `pre_deploy` or `cli`) | [kv/namespaces.md](./kv/namespaces.md) |

---

## Related

- [deployment-sequence.md](./deployment-sequence.md) — demands, `deployment_seq`, deploy waves
- [cli/job-command.md](./cli/job-command.md) — hook events and patterns
- [cli/build.md](./cli/build.md) · [cli/deploy.md](./cli/deploy.md)
- [resources-and-placement.md](resources-and-placement.md)
- [guides/job-commands-tutorial.md](../guides/job-commands-tutorial.md)
