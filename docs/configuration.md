# Configuration reference

Maand reads configuration from the **bucket root** and **`workspace/`**. Edit these files, then run **`maand build`** (and usually **`maand deploy`**) for changes to take effect.

Related: [build.md](./build.md) · [concepts.md](./concepts.md#bucket) · [commands.md](./commands.md#maand-init)

---

## Prerequisites (CLI host and workers)

| Where | Required tools |
|-------|----------------|
| **CLI host** | `maand` (built with `CGO_ENABLED=1`), `bash`, `ssh`, `rsync`, `python3`; `bun` when any job command uses `.ts`/`.js` |
| **Workers** | `python3`, `make`, `rsync`, `bash`, `timeout`; `sudo` when `use_sudo = true` |

SSH: private key in **`secrets/<ssh_key>`** authorized for **`ssh_user`** on every worker. See [tutorials/getting-started.md](./tutorials/getting-started.md).

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
```

| Field | Default | Purpose |
|-------|---------|---------|
| `ssh_user` | `agent` | Remote user for deploy, job control, and `run_command` |
| `ssh_key` | `worker.key` | Private key file under **`secrets/`** |
| `ssh_port` | `22` | SSH port for worker connections and worker health checks |
| `use_sudo` | `true` | Prefix remote rsync and some commands with `sudo` |
| `certs_ttl` | `60` | Days until generated job certs expire |
| `certs_renewal_buffer` | `0` if omitted | Regenerate leaf certs when `NotAfter + buffer` is in the past (use `10` for early renewal) |

Full TLS guide: [certs.md](./certs.md).
| `job_config_selector` | `""` | Suffix for **`bucket.jobs.<selector>.conf`** (see below) |

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
| `port_min` / `port_max` | Inclusive range for maand-assigned job ports |
| Other keys | Copied to KV namespace **`vars/bucket`** at build |

Job manifests declare ports as **`{}`** (assign from pool) or a **fixed integer** (must fall inside `port_min`–`port_max`). See [build.md](./build.md#workspacebucketconf).

---

## `workspace/bucket.jobs.conf` (optional)

Per-job overrides (memory, etc.):

```toml
[api]
memory = "512 mb"
```

| Selector in `maand.conf` | File used |
|--------------------------|-----------|
| `job_config_selector = ""` | `workspace/bucket.jobs.conf` |
| `job_config_selector = "prod"` | `workspace/bucket.jobs.prod.conf` |

Overrides are written to **`vars/bucket/job/<job>`** at build. Only **`memory`** and **`cpu`** keys affect reservations and worker validation; other keys are stored in KV only.

Full guide (manifest bounds, validation, selectors): [resources-and-placement.md](./resources-and-placement.md).

---

## `workspace/jobs/<job>/vars.toml` (optional)

Stable job-scoped variables merged into **`vars/job/<job>`** at build (put-only; keys not listed are preserved). See [kv-variables.md](./kv-variables.md).

---

## `workspace/disabled.json` (optional)

Drain or pause workloads without removing workspace files. Always follow edits with **`maand build`**.

Full guide: [disabled.md](./disabled.md).

---

## `workspace/workers.json`

Worker catalog — hosts, labels, capacity, tags. See [concepts.md](./concepts.md#worker) and [build.md](./build.md#workspaceworkersjson).

---

## `workspace/jobs/<job>/manifest.json`

Job definition — selectors, resources, commands, certs, health. Canonical reference: [jobs-and-dependencies.md](./jobs-and-dependencies.md).

---

## Upgrading configuration after a maand binary upgrade

After installing a newer maand binary, run **`maand init`** before any other command. The CLI checks schema version on every command (except **`init`**) and refuses to run when the database is behind the binary.

```bash
maand init    # schema migrations; keeps bucket_id and CA
maand build
maand deploy
```

If you skip **`init`**, commands such as **`build`** or **`deploy`** print an error like `database schema upgrade required … run maand init to upgrade`.

See [tutorials/day-2-operations.md](./tutorials/day-2-operations.md#upgrade-maand-schema).
