# 7. Build

**`maand build`** reads `workspace/` and updates the local catalog. It does **not** push files to workers.

```bash
maand build
```

Combine with deploy when you want both:

```bash
maand deploy --build
```

---

## What build does

| Step | Result |
|------|--------|
| Sync `workers.json` | `worker` table, labels, tags |
| Sync jobs | Job rows, file index in `job_files`, ports |
| Match selectors | Create/update **allocations** |
| Apply `disabled.json` | Set `disabled` flags on allocations |
| Validate | Resources, ports, circular **demands**, certs |
| Generate TLS | Per-job certs from manifest; auto-rotate per `certs_ttl` |
| Compute `deployment_seq` | Order jobs that depend on each other |
| Run **`post_build`** hooks | Job commands on CLI host, in sequence order |
| Persist KV | Vars from `vars.toml`, template-related state |

Output lives in **`data/maand.db`** and KV — not on workers.

---

## Inspect after build

```bash
maand info
maand cat workers
maand cat jobs              # includes deployment_seq
maand cat allocations
maand cat job_ports --jobs api
maand cat certs --jobs api
```

If build fails, nothing in the catalog advances for that run. Common errors: missing Makefile, port clash, worker too small for job reservation, invalid `workers.json`.

Reference: [build.md](../reference/cli/build.md).

---

## Build vs deploy

| | **build** | **deploy** |
|---|-----------|------------|
| Reads | `workspace/` | `maand.db` + KV |
| Writes | Catalog, certs in bucket | Worker filesystem + lifecycle |
| SSH to workers | No (hooks on CLI host only) | Yes |
| Safe in CI without workers | Yes | No (needs SSH) |

Typical loop: edit workspace → **build** (validate) → **deploy** (push).

---

## Job dependencies (`demands`)

If job `api` declares a **demand** on job `database` command `command_schema`, build assigns **`deployment_seq`** so `database` deploys in an earlier wave than `api`.

```text
deployment_seq 0:  database, cache
deployment_seq 1:  api
```

Reference: [deployment-sequence.md](../reference/deployment-sequence.md).

---

## Certificates

Jobs declare `certs` in the manifest. Build generates or renews TLS material under the bucket and indexes it for deploy. Workers receive certs under `jobs/<job>/certs/` on rsync.

Inspect expiry:

```bash
maand cat certs --jobs api
```

Reference: [certs.md](../reference/certs.md).

---

## Next

[08 — Deploy](./08-deploy.md) — rsync and rolling lifecycle on workers.
