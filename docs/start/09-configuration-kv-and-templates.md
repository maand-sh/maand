# 9. Configuration, KV, and templates

Maand layers configuration so job manifests stay portable while environment-specific values live in the bucket.

---

## Four layers (mental model)

```text
1. manifest.json          job defaults (version, selectors, resources bounds)
2. bucket.jobs*.conf      per-env CPU/memory reservations
3. vars.toml + hooks      job variables and secrets (KV)
4. .tpl templates         per-allocation rendered files at deploy
```

| Layer | Edited by | Read at |
|-------|-----------|---------|
| Manifest | You in git | build |
| `bucket.jobs.conf` / `bucket.jobs.prod.conf` | You in git | build (`job_config_selector` in `maand.conf`) |
| KV `vars/job/<job>` | hooks or `maand jobcommand` | deploy templates, hooks |
| `.tpl` files | You in git | deploy (rendered per worker) |

---

## KV namespaces (intro)

| Namespace | Scope | Example |
|-----------|-------|---------|
| `maand/bucket` | Whole bucket | Shared ports, bucket metadata |
| `maand/worker/<ip>` | One worker | Worker-specific facts |
| `maand/job/<job>/worker/<ip>` | One allocation | `peer_workers`, `version` |
| `vars/job/<job>` | Job variables | Plaintext config |
| `secrets/job/<job>` | Job secrets | Encrypted at rest |

Inspect:

```bash
maand cat kv --jobs api
maand cat kv get vars/job/api db_url
```

Reference: [kv/namespaces.md](../reference/kv/namespaces.md) · [kv/persistence.md](../reference/kv/persistence.md).

---

## Templates (`.tpl`)

Go templates rendered at deploy into the worker tree:

```yaml
# config.yaml.tpl
listen: {{ .WorkerIP }}:{{ get "maand/job/api/worker" .WorkerIP "http_port" }}
peers: {{ get "maand/job/api" "peer_workers" }}
```

Helpers include **`get`**, **`getSecret`**, **`.WorkerIP`**, **`.CurrentVersion`**, **`.NewVersion`**.

Each allocation gets files rendered with **that worker's** context.

Reference: [templates.md](../reference/templates.md).

---

## Secrets workflow

1. **`pre_deploy`** hook: `put_job_secret("db_password", "...")` on CLI host.
2. Deploy renders templates with `getSecret`.
3. Secrets never land in workspace git — stored encrypted in `maand.db`.

Health check hooks cannot write KV (read-only).

---

## Environment-specific reservations

`maand.conf`:

```toml
job_config_selector = "prod"
```

Loads **`workspace/bucket.jobs.prod.conf`** for actual memory/CPU assigned to jobs, while manifest keeps min/max bounds.

Reference: [resources-and-placement.md](../reference/resources-and-placement.md) · [configuration.md](../reference/configuration.md).

---

## `vars.toml` in the job directory

Optional file co-located with the job in workspace — synced to KV at build for non-secret defaults.

---

## Next

[10 — Job commands (hooks)](./10-job-commands.md) — scripts that read and write this state.
