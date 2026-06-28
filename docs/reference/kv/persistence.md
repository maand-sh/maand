# KV persistence and access control

When KV changes are written to **`maand.db`**, who can read/write which namespaces, and how purge/GC works.

**Key reference and examples:** [namespaces.md](./namespaces.md) · **Index:** [README.md](./README.md)

Inspect:

```bash
maand cat kv
maand cat kv --jobs api --active
maand cat kv get --reveal secrets/job/api db_password
```

---

## Namespace overview

| Namespace | Written by | Survives rebuild? | Notes |
|-----------|------------|-------------------|-------|
| `maand/bucket` | build | yes (synced) | Global: `bucket_id`, `jobs`, `activejobs`, port names |
| `maand/worker/<ip>` | build | yes | Worker metadata, labels, peers |
| `maand/worker/<ip>/tags/<key>` | build | yes | From `workers.json` tags |
| `maand/job/<job>` | build | yes (when job active) | Job metadata, `version`, `deploy_order`, workers |
| `maand/job/<job>/worker/<ip>` | build + deploy | yes (when alloc active) | Certs, peers, version |
| `vars/bucket` | build | yes | From `bucket.conf` |
| `vars/bucket/job/<job>` | build | yes (when job active) | From `bucket.jobs*.conf` |
| `vars/job/<job>` | build + job commands | **yes** | App config; not wiped on rebuild |
| `secrets/job/<job>` | job commands | **yes** | AES-256-GCM encrypted |
| `maand/prometheus` | build | yes (synced) | **Scrape configs only** — alerts/runbooks/dashboards stay in `job_files` — [prometheus](../../guides/prometheus.md) |

When a job has **no active allocations**, build and deploy purge build-owned namespaces; **`vars/job`** and **`secrets/job`** are retained unless **`maand build --purge-job-kv`** or deploy reconcile removes them. **`maand gc`** purges remainder. See [cli/gc.md](../cli/gc.md).

---

## Who can read which namespaces

**Job commands** and **templates** on allocation `(job, worker_ip)` may read:

- `maand/bucket`, `vars/bucket`
- `maand/worker`, `maand/worker/<ip>`, `maand/worker/<ip>/tags`
- `maand/job/<job>`, `vars/bucket/job/<job>`, `vars/job/<job>`, `secrets/job/<job>`
- `maand/job/<job>/worker/<ip>`
- Upstream jobs in command **demands**: `maand/job/<upstream>`, `vars/job/<upstream>`, `secrets/job/<upstream>`

**`maand cat kv --jobs <job>`** lists the same union across all non-removed allocations.

Writes from job commands are limited to **`vars/job/<current job>`** and **`secrets/job/<current job>`**. Full matrix: [job-command-api.md](../job-command-api.md#kv-read-vs-write).

---

## Persistence timing

| Context | When writes hit `maand.db` |
|---------|---------------------------|
| **`maand build`** | End of main transaction; **`post_build`** hooks persist in a follow-up transaction |
| **`maand deploy`** | After each job's `pre_deploy` and after each `deployJob` (KV checkpoint) |
| **`maand job_command`** | On successful CLI exit |
| **`maand health_check`** | Read-only (mutations rejected) |

Deploy checkpoints roll back if the deploy transaction aborts before commit. Partial deploy commits KV for successful jobs.

---

## Version history

KV keeps multiple versions per key. **`maand gc --retain-days N`** trims deleted history. Latest active keys: **`maand cat kv --active`**.

---

## Related

- [namespaces.md](./namespaces.md) — keys, examples, cookbook
- [job-command-api.md](../job-command-api.md)
- [templates.md](../templates.md)
- [cli/build.md](../cli/build.md)
