# KV store

Maand keeps configuration and secrets in an in-memory **KV store** loaded from **`maand.db`** (`key_value` table). **Build**, **deploy**, and **job commands** read and write KV; **templates** (`.tpl`) read KV at deploy time.

Inspect from the CLI:

```bash
maand cat kv
maand cat kv --jobs api
maand cat kv --jobs api --active
maand cat kv get maand/job/api version
maand cat kv get --reveal secrets/job/api db_password
```

Related: [job-command.md](./job-command.md) · [templates.md](./templates.md) · [build.md](./build.md#kv-namespaces-build-output)

---

## Namespace overview

| Namespace | Written by | Survives rebuild? | Notes |
|-----------|------------|-------------------|-------|
| `maand` | build | yes (synced) | Global: `bucket_id`, `jobs`, port names |
| `maand/worker/<ip>` | build | yes | Worker metadata, labels, peers |
| `maand/worker/<ip>/tags/<key>` | build | yes | From `workers.json` tags |
| `maand/job/<job>` | build | yes (when job active) | Job metadata, `version`, ports |
| `maand/job/<job>/worker/<ip>` | build + deploy | yes (when alloc active) | Certs, peers, `version` |
| `vars/bucket` | build | yes | From `bucket.conf` |
| `vars/bucket/job/<job>` | build | yes (when job active) | From `bucket.jobs*.conf` |
| `vars/job/<job>` | build (`vars.toml`) + job commands | **yes** | App config; not wiped on rebuild |
| `secrets/job/<job>` | job commands | **yes** | AES-256-GCM encrypted |

When a job has **no active allocations**, build and deploy purge build-owned namespaces; **`vars/job`** and **`secrets/job`** are retained unless **`maand build --purge-job-kv`** or deploy reconcile removes them. **`maand gc`** purges remainder. See [gc.md](./gc.md).

---

## `vars/job/<job>`

Stable application configuration for templates and job commands.

**Populate:**

1. **`workspace/jobs/<job>/vars.toml`** — merged at build (put-only).
2. **Job commands** — `put_job_variable` / `kv_put` in `pre_deploy`, `post_deploy`, `post_build`, or `maand job_command`.

```toml
# workspace/jobs/api/vars.toml
cluster_name = "prod"
```

```python
maand.kv.put("mykey", "myvalue")  # vars/job/<job>
```

---

## `secrets/job/<job>`

Encrypted secrets. Write from job commands only (`put_job_secret`); read in templates via **`getSecret`** or in commands via the runtime API. Never store plaintext secrets in the workspace.

---

## Per-allocation keys

Under **`maand/job/<job>/worker/<ip>`**:

| Key | Meaning |
|-----|---------|
| `certs/*` | TLS material from manifest |
| `<job>_allocation_index` | Index among job peers |
| `is_primary` / `is_seed` | Placement hints |
| `peer_workers` / `peer_ports` | Peer discovery |
| `version` | Target version from build (`allocations.new_version`) |

---

## Who can read which namespaces

**Job commands** and **templates** on allocation `(job, worker_ip)` may read:

- `maand`, `vars/bucket`
- `maand/worker`, `maand/worker/<ip>`, `maand/worker/<ip>/tags`
- `maand/job/<job>`, `vars/bucket/job/<job>`, `vars/job/<job>`, `secrets/job/<job>`
- `maand/job/<job>/worker/<ip>`
- Upstream jobs referenced in command **demands**: `maand/job/<upstream>`, `vars/job/<upstream>`, `secrets/job/<upstream>`

**`maand cat kv --jobs <job>`** lists the same union across all non-removed allocations for that job.

---

## Persistence timing

| Context | When writes hit `maand.db` |
|---------|---------------------------|
| **`maand build`** | End of main transaction; **`post_build`** hooks persist in a follow-up transaction |
| **`maand deploy`** | After each job's `pre_deploy` and after each `deployJob` (checkpoint) |
| **`maand job_command`** | On successful CLI exit |
| **`maand health_check`** | Read-only for KV writes (mutations rejected) |

Full API: [job-command.md](./job-command.md#runtime-http-api).

---

## Version history

KV keeps multiple versions per key. **`maand gc --retain-days N`** trims deleted history. Latest active keys: **`maand cat kv --active`**.
