# Configuration reference

Maand reads configuration from the **bucket root** and **`workspace/`**. Edit these files, then run **`maand build`** (and usually **`maand deploy`**) for changes to take effect.

Related: [build.md](cli/build.md) · [concepts.md](../start/concepts.md#bucket) · [commands.md](cli/commands.md#maand-init)

---

## Prerequisites (CLI host and workers)

| Where | Required tools |
|-------|----------------|
| **CLI host** | `maand` (built with `CGO_ENABLED=1`), `bash`, `ssh`, `rsync`, `python3`; `bun` when any job command uses `.ts`/`.js` |
| **Workers** | `python3`, `make`, `rsync`, `bash`, `timeout`; `sudo` when `use_sudo = true` |

SSH: private key in **`secrets/<ssh_key>`** authorized for **`ssh_user`** on every worker. See [quickstart](../start/quickstart.md).

---

## `maand.conf` (bucket root)

SSH, certificate, and job-config selector settings. Created on first **`maand init`**; not overwritten on later inits.

```toml
ssh_user = "agent"
ssh_key = "worker.key"
ssh_port = 22
use_sudo = true
certs_ttl = 60
certs_renewal_buffer = 10
job_config_selector = ""
log_format = "kv"
```

| Field | Default | Purpose |
|-------|---------|---------|
| `ssh_user` | `agent` | Remote user for deploy, job control, and `run_command` |
| `ssh_key` | `worker.key` | Private key file under **`secrets/`** |
| `ssh_port` | `22` | SSH port for worker connections and worker health checks |
| `use_sudo` | `true` | Prefix remote rsync and some commands with `sudo` |
| `certs_ttl` | `60` | Days until generated job certs expire |
| `certs_renewal_buffer` | `0` if omitted | Regenerate leaf certs when within this many days of expiry (`0` = only after `NotAfter`) |
| `job_config_selector` | `""` | Suffix for **`bucket.jobs.<selector>.conf`** (see below) |
| `log_format` | `kv` | Bucket log encoding: **`kv`**, **`json`**, or **`jsonl`** (JSON lines) |

Full TLS guide: [certs.md](certs.md).

Path on disk: **`<bucket>/maand.conf`**. Worker key path used at runtime: **`secrets/<ssh_key>`**.

---

## `workspace/bucket.conf`

Bucket-wide settings (TOML). Created on **`maand init`** with a default port pool:

```toml
port_min = "30000"
port_max = "39999"
my_setting = "value"
```

| Key | Purpose |
|-----|---------|
| `port_min` / `port_max` | Inclusive pool for maand-assigned (`{}`) job ports only; fixed manifest ports are not limited to this range |
| Other keys | Copied to KV namespace **`vars/bucket`** at build |

Job manifests declare ports as **`{}`** (assign from pool) or a **fixed integer** (any port number). See [build.md](cli/build.md#workspacebucketconf).

---

## Job memory and CPU: manifest bounds vs bucket overrides

Memory and CPU are configured in **two places**; **`maand.conf`** picks which override file applies to the current environment.

| Layer | File | Role |
|-------|------|------|
| **Bounds** | `workspace/jobs/<job>/manifest.json` → `resources.memory` / `resources.cpu` | **Min** and **max** the job is allowed to use (checked into git with the job) |
| **Reservation** | `workspace/bucket.jobs.conf` or `workspace/bucket.jobs.<selector>.conf` | **Actual** memory/CPU for this bucket/environment — must be **within** manifest min/max |
| **Environment pick** | `maand.conf` → `job_config_selector` | Chooses which `bucket.jobs*.conf` file build reads |

Example manifest bounds:

```json
"resources": {
  "memory": { "min": "256 mb", "max": "2 gb" },
  "cpu": { "min": "500 mhz", "max": "2000 mhz" }
}
```

Example reservation for job `api` in the current environment:

```toml
[api]
memory = "512 mb"
cpu = "1500 mhz"
```

Build stores the reservation as **`current_memory_mb`** / **`current_cpu_mhz`** and validates:

```text
manifest min ≤ bucket.jobs value ≤ manifest max
```

Worker capacity in **`workers.json`** must cover the sum of reservations on each host.

---

## `workspace/bucket.jobs.conf` (optional)

Per-job TOML sections. **`memory`** and **`cpu`** set the reservation for that job; other keys are copied to KV only.

```toml
[api]
memory = "512 mb"
cpu = "1500 mhz"
my_setting = "staging-only"
```

### Environment selector (`job_config_selector`)

Set **`job_config_selector`** in **`maand.conf`** to switch environments without editing job manifests. Build loads **one** override file:

| `job_config_selector` in `maand.conf` | Override file read |
|---------------------------------------|--------------------|
| `""` (default) | `workspace/bucket.jobs.conf` |
| `"prod"` | `workspace/bucket.jobs.prod.conf` |
| `"staging"` | `workspace/bucket.jobs.staging.conf` |

Pattern: **`bucket.jobs.<selector>.conf`** where `<selector>` matches `job_config_selector`.

Example layout:

```text
workspace/
├── bucket.jobs.conf           # default / dev reservations
├── bucket.jobs.staging.conf   # used when job_config_selector = "staging"
└── bucket.jobs.prod.conf      # used when job_config_selector = "prod"
```

Change **`job_config_selector`**, then **`maand build`**. Manifest min/max stay the same; only the active reservation file changes.

Overrides are written to **`vars/bucket/job/<job>`** at build. Only **`memory`** and **`cpu`** affect reservations and worker validation.

Full guide (validation, placement labels, examples): [resources-and-placement.md](resources-and-placement.md).

---

## `workspace/jobs/<job>/vars.toml` (optional)

Stable job-scoped variables merged into **`vars/job/<job>`** at build (put-only; keys not listed are preserved). See [KV namespaces](kv/namespaces.md).

---

## `workspace/disabled.json` (optional)

Drain or pause workloads without removing workspace files. Always follow edits with **`maand build`**.

Full guide: [disable and drain](../guides/disable-and-drain.md).

---

## `workspace/workers.json`

Worker catalog — hosts, labels, capacity, tags. See [concepts.md](../start/concepts.md#worker) and [build.md](cli/build.md#workspaceworkersjson).

---

## `workspace/jobs/<job>/manifest.json`

Job definition — selectors, resources, commands, certs, health. Canonical reference: [manifest.md](./manifest.md).

---

## Upgrading configuration after a maand binary upgrade

After installing a newer maand binary, run **`maand init`** before any other command. The CLI checks schema version on every command (except **`init`**) and refuses to run when the database is behind the binary.

```bash
maand init    # schema migrations; keeps bucket_id and CA
maand build
maand deploy
```

If you skip **`init`**, commands such as **`build`** or **`deploy`** print an error like `database schema upgrade required … run maand init to upgrade`.

See [day-2 operations](../guides/day-2-ops.md#upgrade-maand-schema).
