# KV namespaces and variables

Practical guide to **global**, **worker**, **job**, and **allocation** KV — what keys exist, how to set them, and how to use them in templates and job commands.

Maand stores configuration in a **KV store** (SQLite `key_value` table). Values are grouped into **namespaces**. Think in four layers:

| Layer | Who sets it | Typical use |
|-------|-------------|-------------|
| **Global (bucket)** | `bucket.conf`, build | Settings shared by every job |
| **Worker** | `workers.json`, build | Host capacity, labels, peers |
| **Job** | manifest, `vars.toml`, `bucket.jobs.conf`, build, hooks | App config, resource limits |
| **Allocation** | build + deploy | Per (job × worker): certs, peer list, target version |

**Read** values with **`maand cat kv`**, **templates** (`get`), or **job commands** (`maand.kv.get`).  
**Write** user config via workspace files or job-command hooks — not by editing `maand.db`.

Persistence and purge: [persistence.md](./persistence.md) · Index: [README.md](./README.md).

---

## Namespace map

```text
GLOBAL
  maand/bucket                   ← build: bucket_id, jobs, activejobs, job port names (active allocations)
  vars/bucket                    ← bucket.conf (your keys)

WORKER
  maand/worker                   ← build: per-label worker lists, CA cert
  maand/worker/<ip>              ← build: one host’s metadata
  maand/worker/<ip>/tags/<key>   ← build: workers.json tags

JOB
  maand/job/<job>                ← build: catalog metadata (synced)
  maand/prometheus               ← build: scrape catalog only (when prometheus server job exists)
  vars/bucket/job/<job>          ← bucket.jobs*.conf section (synced)
  vars/job/<job>                 ← vars.toml + hooks (merge, not wiped)
  secrets/job/<job>              ← hooks only (encrypted)

ALLOCATION (job on one worker)
  maand/job/<job>/worker/<ip>    ← build + deploy: certs, peers, version
```

### Build-owned vs user-owned

| Namespace | On rebuild | Stale keys removed? |
|-----------|------------|-------------------|
| `maand/bucket`, `maand/worker*`, `maand/job/<job>` | Refreshed from workspace/DB | **Yes** (`syncKeyValues`) |
| `vars/bucket`, `vars/bucket/job/<job>` | Refreshed from TOML | **Yes** |
| `vars/job/<job>` | `vars.toml` merged; hook keys kept | **No** (put-only merge) |
| `secrets/job/<job>` | Unchanged unless hooks/GC | **No** |

---

## Global (bucket) variables

### `vars/bucket` — your bucket-wide settings

**Source:** `workspace/bucket.conf` (TOML).

```toml
port_min = "30000"
port_max = "39999"
environment = "production"
log_level = "info"
```

After **`maand build`**, every key lands in **`vars/bucket`**.

```bash
maand cat kv get vars/bucket environment
# production
```

Use in templates:

```text
{{ get "vars/bucket" "log_level" }}
```

### `maand/bucket` — system global metadata

**Source:** build (not edited directly).

| Key | Example | Meaning |
|-----|---------|---------|
| `bucket_id` | `a1b2c3…` | Bucket UUID |
| `jobs` | `api,worker` | Comma-separated job names |
| `activejobs` | `api` | Comma-separated job names with at least one **active** allocation |
| `<port_name>` | `30042` | Assigned port for jobs with **active** allocations only |

```bash
maand cat kv get maand/bucket bucket_id
maand cat kv get maand/bucket api_http_port
```

Port numbers live **only** in **`maand/bucket`**. Use `get "maand/bucket" "<port_name>"` in templates and job commands (including cross-job reads; no `demands` required).

---

## Worker variables

### `maand/worker/<ip>` — one host

**Source:** `workspace/workers.json` + allocation state at build.

| Key | Meaning |
|-----|---------|
| `worker_ip` | Host IP |
| `worker_id` | Stable worker UUID |
| `hostname` | From `workers.json` if set |
| `position` | Order in `workers.json` |
| `labels` | Comma-separated labels |
| `worker_memory_mb` | Declared memory |
| `worker_cpu_mhz` | Declared CPU |
| `jobs` | Active job names on this worker |
| `<label>_peers` | Other workers with the same label |
| `<label>_allocation_index` | Index among workers with that label |

Example `workers.json`:

```json
[
  {
    "host": "10.0.0.1",
    "hostname": "node-a",
    "labels": ["worker", "api"],
    "memory": "8192 mb",
    "cpu": "4000 mhz",
    "tags": { "zone": "us-east-1a", "rack": "r1" }
  }
]
```

```bash
maand cat kv get maand/worker/10.0.0.1 hostname
maand cat kv get maand/worker/10.0.0.1/tags zone
```

### `maand/worker` — shared across workers (by label)

**Source:** build aggregates workers per label.

| Key pattern | Meaning |
|-------------|---------|
| `<label>_workers` | Comma-separated IPs with that label |
| `<label>_workers_length` | Count |
| `<label>_0`, `<label>_1`, … | IP at index |
| `<label>_label_id` | Stable UUID for the label |
| `certs/ca.crt` | Bucket CA PEM (for deploy) |

```bash
maand cat kv get maand/worker api_workers
# 10.0.0.1,10.0.0.2
```

---

## Job variables

Three namespaces serve different purposes.

### `maand/job/<job>` — catalog metadata (build)

**Source:** `manifest.json`, allocations, resource limits.

| Key | Meaning |
|-----|---------|
| `name`, `job_id` | Job name and UUID |
| `job_name` | Prometheus scrape `job_name` when `_prometheus/scrape.yaml` exists; otherwise the maand job name |
| `version` | Target version from manifest |
| `selectors` | Job selectors |
| `workers`, `workers_length`, `worker_0`, … | **Active** worker IPs (ordered); disabled allocations omitted |
| `deploy_order` | Comma-separated **active** worker IPs for rollout order (synced from catalog on build). Override for one deploy via **`put_deploy_order`** in **`pre_deploy`** or **`cli`** — [job-command-api.md](../job-command-api.md) |
| `memory`, `cpu` | Current reservation |
| `min_memory_mb`, `max_memory_mb`, `min_cpu_mhz`, `max_cpu_mhz` | Manifest bounds |

```bash
maand cat kv --jobs api
maand cat kv get maand/job/api workers
maand cat kv get maand/bucket api_http_port
```

Template (port from global namespace):

```text
{{ get "maand/bucket" "api_http_port" }}
```

### `vars/bucket/job/<job>` — bucket overrides per job

**Source:** `workspace/bucket.jobs.conf` (or `bucket.jobs.<env>.conf` when `job_config_selector` is set).

```toml
[api]
memory = "512 mb"
cpu = "1500 mhz"
replicas_hint = "3"
```

`memory` / `cpu` also drive **`maand/job/api`** reservation fields. Other keys are KV-only.

```bash
maand cat kv get vars/bucket/job/api replicas_hint
```

See [resources-and-placement.md](../resources-and-placement.md).

### `vars/job/<job>` — application config (yours)

**Sources:**

1. **`workspace/jobs/<job>/vars.toml`** — checked in, merged at build.
2. **Job commands** — `put_job_variable` / `maand.kv.put` in hooks.

```toml
# workspace/jobs/api/vars.toml
cluster_name = "prod-east"
feature_flags = "tls,v2"
```

```python
# _modules/command_setup.py (e.g. post_build)
import maand

def main():
    maand.put_job_variable("schema_version", "12")
```

Rebuild merges `vars.toml` keys **without deleting** keys added only by hooks.

```bash
maand cat kv get vars/job/api cluster_name
```

Template:

```text
{{ get "vars/job/api" "cluster_name" }}
```

### `secrets/job/<job>` — encrypted secrets

Write only from job commands (`put_job_secret`). Read with **`getSecret`** in templates or the runtime API.

```python
maand.put_job_secret("db_password", "s3cret")
```

```bash
maand cat kv get --reveal secrets/job/api db_password
```

Never put secrets in `vars.toml` or the workspace.

---

## Allocation variables (job × worker)

Namespace: **`maand/job/<job>/worker/<ip>`**

| Key | Set by | Meaning |
|-----|--------|---------|
| `certs/<name>.crt`, `.key` | build | TLS material — [certs.md](../certs.md) |
| `<job>_allocation_index` | build | Index among **non-removed** job peers |
| `peer_workers` | build | Comma-separated peer IPs for this job (**non-removed**, includes disabled) |
| `version` | build + deploy | Target deploy version for this allocation |

```bash
maand cat kv get maand/job/api/worker/10.0.0.1 peer_workers
maand cat kv get maand/job/api/worker/10.0.0.1 api_allocation_index
```

Template (rendered per allocation):

```text
{{ get "maand/job/api/worker/10.0.0.1" "version" }}
```

Use template context **`.WorkerIP`** so one `.tpl` works on every worker:

```json
{
  "peers": "{{ get (printf "maand/job/api/worker/%s" .WorkerIP) "peer_workers" }}",
  "target_version": "{{ .NewVersion }}"
}
```

---

## End-to-end example

**Layout**

```text
workspace/
├── bucket.conf
├── bucket.jobs.conf
├── workers.json
└── jobs/
    └── api/
        ├── manifest.json
        ├── vars.toml
        ├── Makefile
        └── config.json.tpl
```

**`bucket.conf`**

```toml
environment = "staging"
port_min = "30000"
port_max = "39999"
```

**`workers.json`**

```json
[
  { "host": "10.0.0.1", "labels": ["worker", "api"], "memory": "4096 mb", "cpu": "2000 mhz" },
  { "host": "10.0.0.2", "labels": ["worker", "api"], "memory": "4096 mb", "cpu": "2000 mhz" }
]
```

**`manifest.json`**

```json
{
  "version": "1.2.0",
  "selectors": ["worker", "api"],
  "resources": {
    "memory": { "min": "256 mb", "max": "1 gb" },
    "ports": { "api_http_port": {} }
  }
}
```

**`bucket.jobs.conf`**

```toml
[api]
memory = "512 mb"
```

**`vars.toml`**

```toml
service_name = "api-gateway"
```

**`config.json.tpl`**

```json
{
  "env": "{{ get "vars/bucket" "environment" }}",
  "service": "{{ get "vars/job/api" "service_name" }}",
  "listen": "{{ get "maand/bucket" "api_http_port" }}",
  "peers": "{{ get (printf "maand/job/api/worker/%s" .WorkerIP) "peer_workers" }}",
  "version": "{{ .NewVersion }}"
}
```

**Build and inspect**

```bash
maand build

maand cat kv get vars/bucket environment          # staging
maand cat kv get maand/job/api workers              # 10.0.0.1,10.0.0.2
maand cat kv get vars/job/api service_name          # api-gateway
maand cat kv get maand/worker/10.0.0.1 worker_memory_mb
maand cat kv get maand/job/api/worker/10.0.0.1 peer_workers   # 10.0.0.2
```

**Deploy** renders `config.json.tpl` per worker using that worker’s allocation namespace, then rsyncs to `/opt/worker/<bucket_id>/jobs/api/`.

**Runtime update from a hook** (`pre_deploy`):

```python
import maand

def main():
    maand.put_job_variable("deployed_at", maand.env("EVENT"))
```

Persisted when deploy checkpoints KV for that job.

---

## Quick reference: where to put config

| I want… | Put it in… | Namespace |
|---------|------------|-----------|
| Bucket-wide setting | `workspace/bucket.conf` | `vars/bucket` |
| Per-job bucket override | `workspace/bucket.jobs*.conf` | `vars/bucket/job/<job>` |
| App config in git | `workspace/jobs/<job>/vars.toml` | `vars/job/<job>` |
| Secret | job command hook | `secrets/job/<job>` |
| Port numbers | `manifest.json` + build | `maand/bucket` |
| Workers / version | `manifest.json` + build | `maand/job/<job>` |
| Host metadata | `workers.json` + build | `maand/worker/<ip>` |
| Peers / certs on one node | automatic at build/deploy | `maand/job/<job>/worker/<ip>` |

---

## Inspecting KV

```bash
maand cat kv                              # all namespaces (truncated values)
maand cat kv --jobs api                   # job-related namespaces
maand cat kv --jobs api --workers 10.0.0.1
maand cat kv --active                     # latest non-deleted versions only
maand cat kv get maand/job/api version
maand cat kv get --reveal secrets/job/api my_secret
```

---

## Related

- [persistence.md](./persistence.md) — persistence, purge, access control
- [templates.md](../templates.md) — `get`, `getSecret`, template context
- [job-command-api.md](../job-command-api.md) — runtime API for reads/writes
- [configuration.md](../configuration.md) — `bucket.conf`, `bucket.jobs.conf`, `vars.toml`
