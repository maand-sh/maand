# `maand build`

**Build** reads your workspace from disk, updates the bucket database (`maand.db`), fills the in-memory **KV store** (then persists it), and generates TLS material. It does **not** contact workers.

---

## CLI

```bash
maand build
```

| Flag | Description |
|------|-------------|
| *(none)* | Full build |

There is no job or worker filter on build; the entire workspace is reconciled every time.

---

## Prerequisites

1. **`maand init`** in the bucket directory (creates DB schema, default `maand.conf`, CA, SSH key, `tmp/`).
2. **`CGO_ENABLED=1`** when compiling maand (SQLite driver).
3. **Host tools** for `post_build` job commands: `python3`, `bun` (if using Bun commands), `bash`, `ssh`, `rsync`.
4. Valid **`workspace/workers.json`** and at least one job under **`workspace/jobs/<name>/`** with `manifest.json` and **`Makefile`**.

---

## Workspace inputs

### `workspace/workers.json`

Array of workers (hosts are SSH targets from the maand CLI host):

```json
[
  {
    "host": "10.0.0.1",
    "labels": ["worker", "gpu"],
    "memory": "4096 mb",
    "cpu": "2000 mhz",
    "tags": { "zone": "a" }
  }
]
```

- **`host`**: Worker IP or hostname (required, unique).
- **`labels`**: Used for job **selectors** (label matching). The label `worker` is always added automatically.
- **`memory` / `cpu`**: Parsed to MB / MHz; stored on the worker row and exposed in KV.
- **`tags`**: Arbitrary key/value strings → namespace `maand/worker/<ip>/tags/<key>`.
- **`position`**: Ordering field (assigned from array index on read).

Workers removed from `workers.json` are marked **`removed`** on allocations (not deleted until deploy/GC).

### `workspace/jobs/<job>/manifest.json`

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "update_parallel_count": 2,
  "resources": {
    "memory": { "min": "128 mb", "max": "512 mb" },
    "cpu": { "min": "100 mhz", "max": "500 mhz" },
    "ports": { "http_port": {} }
  },
  "commands": {
    "command_setup": {
      "executed_on": ["post_build", "pre_deploy"],
      "demands": {
        "job": "other_job",
        "command": "command_base",
        "config": {}
      }
    }
  },
  "certs": {
    "tls": {
      "pkcs8": false,
      "one": true,
      "subject": { "common_name": "app.example" }
    }
  }
}
```

| Field | Purpose |
|-------|---------|
| `version` | Job version string — see [Job version](#job-version) below |
| `selectors` | Worker **labels** required to place the job (all selectors must match worker labels). |
| `update_parallel_count` | Rolling restart batch size during **deploy** (minimum 1). |
| `resources.memory` / `cpu` | Limits; optional override via `bucket.jobs.conf`. |
| `resources.ports` | Named ports (`"database_port": {}`); numbers assigned from `bucket.conf` range at build |
| `commands` | Named commands (must be prefixed `command_`). |
| `certs` | Per-job cert definitions → generated into KV per worker. |

### Job version

Each job may declare **`version`** in `manifest.json`. Build stores it in **`job.version`** and KV (`maand/job/<job>/version`). When omitted, the target version is stored as **`0.0.0`**.

Per-allocation **running** vs **target** versions (`current_version` / `new_version`) are recorded during **deploy** and promoted after a successful rollout — see [deploy.md](./deploy.md#allocation-version-tracking).

**Format** (validated when the field is present or required):

- Semver-like: `major.minor.patch` (missing segments default to `0`, so `"1"` → `1.0.0`)
- Optional leading `v` (`v2.1.0`)
- Optional prerelease suffix (`2.0.0-rc1`)
- Rejected: empty string, `unknown`, more than three numeric segments

**When `version` is required**

| Job role | Rule |
|----------|------|
| Standalone job (no demands, not demanded) | Optional — omit allowed |
| Job with **`demands.job`** set on any command | Must declare **`version`** |
| Upstream job referenced by another job’s demand | Must declare **`version`** |

**Version constraints on dependencies**

In **`demands.config`**, optional bounds are checked against the **upstream job’s `version`** at build:

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

- `min_version` / `max_version`: string (`"2.1.0"`) or integer (`2` → `2.0.0`)
- Build fails with **`ErrJobCommandDemandVersionMismatch`** when upstream version is out of range

Full rules: [jobs-and-dependencies.md](./jobs-and-dependencies.md#job-version) · [jobs-and-dependencies.md](./jobs-and-dependencies.md#what-demands-validate-at-build)

### Job directory rules

- **`Makefile`** is **required** (used by default deploy via `runner.py`: `make start|stop|restart`).
- Reserved directories on disk: **`data/`**, **`logs/`**, **`bin/`** must **not** exist under the job folder (they are created on workers at runtime).
- Job files are copied into table **`job_files`** (content stored in DB).

### `workspace/disabled.json` (optional)

```json
{
  "jobs": {
    "myjob": {
      "allocations": ["10.0.0.1"]
    }
  },
  "workers": ["10.0.0.2"]
}
```

- Disable entire job: `"jobs": { "myjob": {} }`.
- Disable specific workers for a job: `"allocations": ["<worker_ip>", ...]`.
- Disable all jobs on a worker: `"workers": ["<ip>", ...]`.

Disabled allocations are not **active** for deploy/job commands but rows remain until removed from workspace/build.

### `workspace/bucket.jobs.conf` (optional)

TOML map of job name → settings (e.g. memory override):

```toml
[myjob]
memory = "256 mb"
```

If `maand.conf` sets `job_config_selector = "prod"`, the file used is **`bucket.jobs.prod.conf`**.

### `workspace/bucket.conf`

Bucket-wide settings (TOML key/value). Created on **`maand init`** with a default **port pool**:

```toml
port_min = "30000"
port_max = "39999"
```

Job manifests declare named ports as empty objects; **`maand build`** assigns the lowest free port in this inclusive range. Assigned numbers are stored in **`job_ports`** and exposed in KV (`maand/<port_name>`). A port keeps the same number across rebuilds until the job is removed or the port name is removed from the manifest.

```json
"resources": {
  "ports": {
    "database_port": {},
    "http_port": {}
  }
}
```

Port names must be lowercase identifiers (`database_port`, `http_port`). Hardcoded port numbers in manifests are rejected.

Other keys in `bucket.conf` are copied to KV namespace **`vars/bucket`**.

### `workspace/maand.conf`

```toml
ssh_user = "agent"
ssh_key = "worker.key"
use_sudo = false
certs_ttl = 60
certs_renewal_buffer = 10
job_config_selector = ""
```

Written on first init if missing; not overwritten on later inits.

---

## What `build` does (order)

All steps run in one **transaction** (except post-build hooks), then `VACUUM`:

| Step | Function | Summary |
|------|----------|---------|
| 1 | `kv.Initialize` | Load KV from DB into memory. |
| 2 | `BuildWorkers` | Sync `workers.json` → `worker`, labels, tags; drop removed workers from catalog. |
| 3 | `BuildJobs` | Sync each job manifest → `job`, `job_selectors`, `job_commands`, `job_files`, `job_ports`, `job_certs`. |
| 4 | `ValidateJobCommandDemands` | Verify demand job/command refs; parse **`version`**; check **`min_version`** / **`max_version`**. |
| 5 | `BuildAllocations` | Label-match jobs to workers → `allocations` rows (`alloc_id`, `deployment_seq` initially 0). |
| 6 | `BuildDeploymentSequence` | Compute `deployment_seq` from **command demands** (dependency order). |
| 7 | `BuildVariables` | Populate KV namespaces (workers, jobs, per-allocation certs path, bucket vars). |
| 8 | `BuildCerts` | Regenerate CA/job certs when CA or cert config changed; write cert PEMs into KV. |
| 9 | `PurgeStaleVersions` | Trim old KV versions (keep 7 per key). |
| 10 | `ValidateWorkerResources` | Ensure allocated jobs fit worker memory/CPU. |
| 11 | `PersistToTransaction` | Write KV changes into `key_value` table. |
| 12 | **Commit** | Persist catalog. |
| 13 | `runPostBuildHooks` | **Separate transaction**; failures **fail the build**. |

### Deployment sequence (`deployment_seq`)

Derived from **`job_commands.demand_job`** edges. See **[jobs-and-dependencies.md](./jobs-and-dependencies.md)** for the full reference.

Summary:

- Jobs with no upstream demands → sequence **0**.
- If job B’s command demands job A → B’s sequence is greater than A’s (max chain depth if multiple demands).
- **Circular demands** fail the build with `ErrCircularJobCommandDependency`.
- All allocations for a job share the same `deployment_seq`.

Deploy runs sequences **0 .. max** in order so depended-on jobs deploy first. **`post_build`** hooks use the same order.

### Allocations

- One row per **(worker_ip, job)** match.
- **`alloc_id`**: Stable UUID from `hash(job|workerIP)`.
- **`removed`**: Set when worker or job disappears from workspace (cleaned on deploy).
- **`disabled`**: From `disabled.json` or resource validation.

### KV namespaces (build output)

Examples written during build:

| Namespace | Examples |
|-----------|----------|
| `maand/worker/<ip>` | `worker_ip`, `worker_id`, `labels`, `worker_memory_mb`, `jobs`, label peers |
| `maand/worker/<ip>/tags/<key>` | Tag values |
| `maand/job/<job>` | `name`, `version` (target), `selectors`, memory/cpu limits |
| `vars/bucket/job/<job>` | Entries from `bucket.jobs.conf` |
| `vars/job/<job>` | Job-scoped vars for templates |
| `maand/job/<job>/worker/<ip>` | `certs/*`, `current_version`, `new_version` (deploy) |
| `maand/worker` | `certs/ca.crt` |

Templates (`.tpl`) and job commands read these via the runtime API or deploy transpile.

### Certificates

- Bucket **CA** in `secrets/ca.crt` / `ca.key`.
- Per-job certs from manifest → stored in KV under each worker namespace.
- Regenerated when CA hash or `build_certs` job hash changes.
- Manifest `"pkcs8": true` writes PKCS#8 private keys (`PRIVATE KEY` PEM); default is PKCS#1 (`RSA PRIVATE KEY`).
- Removing certs from a job manifest purges `certs/*` KV keys on the next build.

## `post_build` job commands

After the main commit, build runs every command registered with **`executed_on` containing `post_build`**, in **deployment sequence** order on **active allocations** (via SSH, same as deploy hooks).

- Uses `jobcommand.JobCommand` (same as deploy/CLI).
- **Any hook failure fails `maand build`** (main catalog commit already succeeded; fix hooks and re-run build).
- Useful for validation or codegen that must run before deploy.

### Resource validation

If allocated jobs require memory or CPU (`resources.memory` / `resources.cpu` in the manifest), each worker hosting those jobs **must** declare `memory` / `cpu` in `workers.json`. Build fails when requirements exceed capacity or when a worker omits capacity while jobs require it.

---

## Database tables touched

`worker`, `worker_labels`, `worker_tags`, `job`, `job_selectors`, `job_commands`, `job_files`, `job_ports`, `job_certs`, `allocations`, `hash`, `key_value`, `bucket`, `schema_version`.

Inspect with:

```bash
maand cat workers
maand cat jobs
maand cat allocations
maand cat job_commands
maand cat kv
```

---

## Common errors

| Error | Typical cause |
|-------|----------------|
| `ErrInvalidWorkerJSON` | Duplicate host, bad memory/cpu format |
| `ErrInvalidManifest` | Bad resources, missing Makefile |
| `ErrInvalidJobCommandDemand` | Unknown demand job/command or partial demand pair |
| `ErrInvalidJobVersion` | Invalid or missing version on dependency participant |
| `ErrJobCommandDemandVersionMismatch` | Upstream version outside demand min/max |
| `ErrPortKeyFormat` | Invalid port name in manifest |
| `ErrInvalidManifestPort` | Port value is not `{}` (hardcoded numbers rejected) |
| `ErrInvalidPortRange` | Bad `port_min` / `port_max` in `bucket.conf` |
| `ErrPortRangeExhausted` | No free ports left in the pool |
| `ErrCircularJobCommandDependency` | Demand cycle between jobs |
| Worker resource validation | Job memory/CPU exceeds worker capacity, or worker missing memory/CPU when jobs require it |

---

## When to run build

- After changing **workers.json**, **jobs**, **disabled.json**, or **bucket.jobs.conf**.
- Before **`maand deploy`** (or use `maand deploy --build` / `-b`).
- When you need **cert** rotation or catalog refresh before deploy.

Build does **not** start or restart processes on workers; that is **deploy**.

---

## Related commands

- [`deploy.md`](./deploy.md) — push artifacts and roll out.
- [`job-command.md`](./job-command.md) — command scripts and events.
- [`health-check.md`](./health-check.md) — `health_check` event.
