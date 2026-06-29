# 3. Bucket and workspace

A **bucket** is one maand project — a directory you `cd` into before running `maand` commands.

---

## Create a bucket

```bash
mkdir my-cluster && cd my-cluster
maand init
```

`maand init` creates (or upgrades) layout, generates a bucket CA, encryption key for KV, and an empty `workers.json`.

Verify:

```bash
ls
# maand.conf  data/  workspace/  secrets/  tmp/  logs/
```

Each bucket has a stable **`bucket_id`** (UUID). Workers store it in `/opt/worker/<bucket_id>/worker.json` so multiple buckets can coexist on one host under different IDs.

---

## What you edit vs what maand owns

| Path | You edit? | Purpose |
|------|-----------|---------|
| `workspace/` | **Yes** (git) | Workers, jobs, optional `disabled.json`, `bucket.conf` |
| `maand.conf` | **Yes** | SSH user/key, sudo, cert TTL, environment selector |
| `secrets/` | **Carefully** | SSH private key, CA, KV key — not usually in git |
| `data/maand.db` | **No** | SQLite catalog — output of **build** |
| `tmp/` | **No** | Deploy staging (ephemeral) |
| `logs/` | **No** | Maand command logs (deploy, rsync, SSH) |

**Rule of thumb:** if it's under `workspace/`, you change it and run **`maand build`**. If it's deploy output on workers, you change it via **`maand deploy`** (or job Makefile at runtime).

---

## `maand.conf` essentials

```toml
ssh_user = "agent"
ssh_key = "worker.key"      # file under secrets/
use_sudo = true             # wrap remote commands in sudo when needed

# Optional:
job_config_selector = "prod"  # loads bucket.jobs.prod.conf for reservations
log_format = "kv"             # or json / jsonl for logs/*.log
```

Copy your SSH private key:

```bash
cp ~/.ssh/id_ed25519 secrets/worker.key
chmod 600 secrets/worker.key
```

The matching public key must be in `authorized_keys` for `ssh_user` on every worker.

Full reference: [configuration.md](../reference/configuration.md).

---

## Workspace layout

```text
workspace/
├── workers.json           # cluster nodes
├── disabled.json          # optional: drain without delete
├── bucket.conf            # optional: shared ports, settings
├── bucket.jobs.conf       # optional: per-job CPU/memory reservations
├── bucket.jobs.prod.conf  # optional: when job_config_selector = prod
└── jobs/
    └── api/
        ├── manifest.json
        ├── Makefile
        ├── config.tpl     # optional
        └── _modules/      # optional: job command scripts
```

Do **not** put runtime `data/`, `logs/`, or `bin/` inside workspace job folders — build rejects them. Those directories exist **on workers** after deploy.

---

## CLI host requirements

On the machine where you run `maand`:

- `maand` binary (built with `CGO_ENABLED=1` for SQLite)
- `bash`, `ssh`, `rsync`, `python3` (and `bun` if any job command uses it)

On workers (checked at deploy / run_command):

- `bash`, `ssh` server, `rsync`, `python3`, `make`, `timeout` on `PATH`
- `sudo` if `use_sudo = true`

---

## Two phases: catalog vs runtime

```text
workspace  ──build──►  maand.db (catalog)
                           │
                           └──deploy──►  workers (/opt/worker/...)
```

- **Build** never SSHs to workers (except `post_build` hooks run on CLI host only).
- **Deploy** reads the catalog and pushes to workers.

This split lets you validate placement and ports locally before touching production hosts.

---

## Next

[04 — Workers](./04-workers.md) — define your cluster nodes in `workers.json`.
