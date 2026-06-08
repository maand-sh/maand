# `maand build`

**Build** reads your workspace from disk, updates the bucket database (`maand.db`), fills the in-memory **KV store** (then persists it), and generates TLS material. It does **not** contact workers.

---

## CLI

```bash
maand build [--purge-job-kv]
```

| Flag | Description |
|------|-------------|
| `--purge-job-kv` | Mark `vars/job/<job>` and `secrets/job/<job>` deleted when a job has no active allocations |
| *(default)* | Full workspace reconcile (no job or worker filter) |

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
| `resources.ports` | Named ports: `{}` (maand assigns) or integer (fixed); must be within `bucket.conf` range |
| `commands` | Named commands (must be prefixed `command_`). |
| `certs` | Per-job cert definitions → generated into KV per worker. |

### Job version

Each job may declare **`version`** in `manifest.json`. Build stores it in **`job.version`** and KV (`maand/job/<job>/version`). When omitted, the target version is stored as **`0.0.0`**.

Per-allocation **target** version (`allocations.new_version`) is set on **`maand build`** from `manifest.json`. **Running** version (`hash.current_version`) is updated on **deploy** promote — see [deploy.md](./deploy.md#allocation-version-tracking).

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

### Health check (build validation)

A job may define **either**:

- **`health_check`** in `manifest.json` (built-in tcp/http/ssh probes), **or**
- A **`command_*`** with **`executed_on`: `["health_check"]`**

Defining **both** fails build with **`ErrInvalidManifest`**. Manifest probes are stored in **`job.health_check`** (JSON). Command-based health is stored in **`job_commands`** like other events. See [health-check.md](./health-check.md).

### `workspace/disabled.json` (optional)

How-to: [disabled.md](./disabled.md).

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

Job manifests declare named ports in two ways:

- **`{}`** — maand assigns the lowest free port in the inclusive range at build time.
- **An integer** — you pin the port number in the manifest (must be inside `port_min`–`port_max`).

Assigned numbers are stored in **`job_ports`** and exposed in KV (`maand/<port_name>`). Maand-provisioned ports keep the same number across rebuilds until the job is removed or the port name is removed. Fixed ports always follow the manifest value.

```json
"resources": {
  "ports": {
    "database_port": 5432,
    "http_port": {}
  }
}
```

Port names must be lowercase identifiers (`database_port`, `http_port`). The same port number cannot be used by two jobs or two port names in the bucket.

Other keys in `bucket.conf` are copied to KV namespace **`vars/bucket`**.

### `maand.conf`

Bucket-root SSH and cert settings. Full field reference: [configuration.md](./configuration.md#maandconf-bucket-root).

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
| 7 | `BuildVariables` | Populate KV namespaces (workers, jobs, bucket vars, job/allocation metadata). |
| 8 | `BuildCerts` | Regenerate CA/job certs when CA or cert config changed; write cert PEMs into KV. |
| 9 | `BuildJobAllocationVariables` | Per-allocation keys (`*_allocation_index`, `peer_workers`) after certs so cert sync does not delete them. |
| 10 | `PurgeStaleVersions` | Trim old KV versions (keep 7 per key). |
| 11 | `ValidateWorkerResources` | Ensure allocated jobs fit worker memory/CPU. |
| 12 | `PersistToTransaction` | Write KV changes into `key_value` table. |
| 13 | **Commit** | Persist catalog. |
| 14 | `runPostBuildHooks` | **Separate transaction**; runs `post_build` commands, then **persists `vars/job` KV**; failures **fail the build**. |

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

Build populates catalog-backed namespaces (`maand/*`, `vars/bucket/*`, worker and job metadata, certs). Stable app config lives in **`vars/job/<job>`** and **`secrets/job/<job>`**.

Full namespace reference, persistence rules, and purge behavior: [kv.md](./kv.md).

### Certificates

- Bucket **CA** in `secrets/ca.crt` / `ca.key`.
- Per-job certs from manifest → stored in KV under each worker namespace.
- Regenerated when CA hash or `build_certs` job hash changes.
- Manifest `"pkcs8": true` writes PKCS#8 private keys (`PRIVATE KEY` PEM); default is PKCS#1 (`RSA PRIVATE KEY`).
- Removing certs from a job manifest purges `certs/*` KV keys on the next build.

## `post_build` job commands

After the main commit, build runs every command registered with **`executed_on` containing `post_build`**, in **deployment sequence** order on **active allocations** on the **CLI host** (same runtime as deploy hooks).

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
| `ErrInvalidManifestPort` | Port value is not `{}` or a valid integer in range |
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

## Related

- [deploy.md](./deploy.md) — push artifacts and roll out
- [job-command.md](./job-command.md) — command scripts and events
- [kv.md](./kv.md) — namespaces and persistence
- [templates.md](./templates.md) — `.tpl` rendering
- [configuration.md](./configuration.md) — `maand.conf`, `bucket.conf`
- [health-check.md](./health-check.md) — `health_check` event
