# 2. From your background

This chapter maps familiar tools to maand so later chapters feel like extensions of what you already do, not a new religion.

---

## If you know Linux and SSH

You already have most prerequisites.

| What you do today | Maand equivalent |
|-------------------|------------------|
| `ssh user@host` | Same — all worker access uses `secrets/<ssh_key>` and `ssh_user` from `maand.conf` |
| Copy files with `rsync` | `maand deploy` rsyncs staged trees to `/opt/worker/<bucket_id>/` |
| `make start` in a service directory | Job **Makefile**; deploy calls `start` / `restart` / `reload` on the worker |
| `cron` or manual ops scripts | `maand run_command` for ad-hoc remote shell; job commands for repeatable hooks |

Maand adds a **catalog** (who runs what) and **orchestrated rollouts** on top of SSH + Make.

---

## If you know Ansible (or similar config management)

| Ansible | Maand |
|---------|-------|
| Inventory (`hosts`) | `workspace/workers.json` |
| Play per host / role | **Job** + **allocation** (job × worker) |
| `ansible-playbook` push | `maand deploy` (rsync + lifecycle) |
| Variables per host | KV namespaces + Go **templates** (`.tpl`) rendered at deploy |
| Handlers / rolling serial | `update_parallel_count`, `deploy_order`, health between batches |
| Vault for secrets | `secrets/job/<job>` in KV (encrypted); `put_job_secret` in hooks |

Maand is **not** a general-purpose CM tool. It optimizes for **declared jobs on a fixed worker pool** with versioned deploys and hash-based skip, not arbitrary ad-hoc playbooks.

---

## If you know systemd and Docker Compose

Maand does not manage units or containers directly. Your job **Makefile** does:

```makefile
# common pattern
start:
	docker compose up -d

stop:
	docker compose down
```

Or systemd:

```makefile
start:
	sudo systemctl start myapp.service

stop:
	sudo systemctl stop myapp.service
```

Deploy runs `make start` once per new allocation and `make restart` or `make reload` on upgrades (depending on `restart_policy` in the manifest). See [05-jobs-and-lifecycle.md](./05-jobs-and-lifecycle.md).

---

## If you know Kubernetes

| Kubernetes | Maand |
|------------|-------|
| Cluster API server | CLI host + `maand.db` (local SQLite) |
| Node | **Worker** (fixed IP/hostname) |
| Deployment / StatefulSet | **Job** (many **allocations** = one per matching worker) |
| Pod | Your process/container on the worker — outside maand's object model |
| `kubectl apply` | `maand build` then `maand deploy` |
| ConfigMap / Secret | KV + templates; encrypted secrets namespace |
| RollingUpdate | `update_parallel_count` + `maand health_check` between batches |
| PodDisruptionBudget / drain | `disabled.json` + build + deploy |

Maand has **no pod abstraction**. Identity is **(job, worker IP, alloc_id)**. Good fit when you want K8s-*like* rolling deploys without running a cluster control plane.

Deeper comparison: [comparison-orchestrators.md](./comparison-orchestrators.md).

---

## If you know Nomad

| Nomad | Maand |
|-------|-------|
| Nomad server + client agents | No agents — SSH from CLI host |
| Job spec (HCL) | `manifest.json` + files in `workspace/jobs/<job>/` |
| Allocation | Same word: job instance on one worker |
| `nomad job run` | `maand deploy` |
| Consul KV | Maand embedded KV (scoped namespaces) |
| Update stanza (parallel, canary) | `deploy_parallel_count` / `update_parallel_count`, `job_control` hooks |

Nomad scales to dynamic scheduling; maand assumes you **named the workers** in advance.

---

## One diagram, three tools

```text
         Ansible              Kubernetes           Maand
         ───────              ──────────           ─────
Inventory / vars      →      API + etcd        →   workspace + maand.db
Play / role           →      Deployment        →   job
Host in group         →      Pod on node       →   allocation (job @ worker)
Push + handlers       →      reconcile loop    →   build + deploy (explicit CLI)
```

---

## Next

[03 — Bucket and workspace](./03-bucket-and-workspace.md) — where files live and what you edit in git.
