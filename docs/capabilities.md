# Capabilities overview

Maand is an **agentless workload orchestrator** for Linux clusters — more than a simple deploy script, but narrower in scope than Kubernetes or Nomad.

A **CLI host** (laptop, CI runner, or bastion) holds a local **bucket**: SQLite catalog, encrypted KV, secrets, and staged files. **Workers** are ordinary Linux hosts reached over **SSH** and **rsync**. Nothing maand-specific is installed on workers except what you deploy (`runner.py`, job files, Makefiles).

That design fits **small-to-medium fleets** where you want declarative jobs, rolling deploys, and ops hooks without running a separate control-plane cluster.

Related: [concepts.md](./concepts.md) · [commands.md](./commands.md) · [configuration.md](./configuration.md) · [getting started](./tutorials/getting-started.md)

---

## Core capabilities

| Area | What maand can do |
|------|-------------------|
| **Catalog & placement** | Declare workers (`workers.json`) with labels, tags, CPU/memory; declare jobs with **selectors** (label matching); auto-create **allocations** (job × worker). Three-layer **resource model**: manifest min/max, `bucket.jobs*.conf` reservations, worker capacity validation — see [resources-and-placement.md](./resources-and-placement.md) |
| **Environment overrides** | `job_config_selector` in `maand.conf` picks `bucket.jobs.<env>.conf` for per-environment memory/CPU without changing manifests — see [configuration.md](./configuration.md) |
| **Build** | Reconcile workspace → `maand.db` + KV; validate resources, ports, job dependencies; generate and **auto-rotate TLS** certs; sync `disabled.json`; run **`post_build`** hooks in `deployment_seq` order — see [build.md](./build.md), [certs.md](./certs.md) |
| **Deploy** | Rsync to `/opt/worker/<bucket_id>/`; Makefile start/stop/restart (or custom **`job_control`**); **hash-based skip** for unchanged jobs; **partial deploy resume**; **`--dry-run`**, **`--force`**, **`--jobs`** filters; ordered **waves** via `deployment_seq` — see [deploy.md](./deploy.md), [deploy-debugging.md](./deploy-debugging.md) |
| **Rolling upgrades** | `update_parallel_count` for batched restarts with health gates between batches; semver **`version`** tracking (`CURRENT_VERSION` / `NEW_VERSION` in Makefiles and job commands); Makefile migration hooks — see [rolling-upgrade.md](./rolling-upgrade.md) |
| **Dependencies** | Cross-job **command demands** with version constraints; build computes **`deployment_seq`** and detects circular demands — see [jobs-and-dependencies.md](./jobs-and-dependencies.md) |
| **Job commands & runtime API** | Python/Bun hooks on the **CLI host** for `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, and ad-hoc **`cli`**. In-process HTTP API (`localhost:8080`) for KV read/write, **`GET /demands`**, and **cross-allocation semaphores**. Python helpers for SSH to workers — see [job-command.md](./job-command.md) |
| **Health** | Built-in TCP/HTTP/SSH manifest probes, or custom **`health_check`** commands (mutually exclusive per job); **`--wait`**, **`--update-hash`** to trigger redeploy — see [health-check.md](./health-check.md) |
| **Maintenance (disable)** | **`disabled.json`** drains workers, jobs, or single allocations **without** removing workspace or catalog rows. Disabled allocations still get build/KV/certs and deploy staging; maand **never starts** them — see [disabled.md](./disabled.md) |
| **Day-2 ops** | `maand job start\|stop\|restart\|run\|status`; `maand run_command` for ad-hoc SSH; soft-remove via workspace edits + **GC** for purged rows and disk — see [job.md](./job.md), [gc.md](./gc.md), [tutorials/day-2-operations.md](./tutorials/day-2-operations.md) |
| **State & secrets** | Layered KV: global (`vars/bucket`), worker, job (`vars.toml` + hooks), allocation (certs, peers, version). Encrypted **`secrets/job/*`**. Inspect with **`maand cat`** — see [kv.md](./kv.md), [kv-variables.md](./kv-variables.md) |
| **TLS** | Bucket CA at init; per-job certs declared in manifest; build-time renewal via `certs_ttl` / `certs_renewal_buffer`; staged to workers on deploy — see [certs.md](./certs.md) |
| **Templates** | Go templates (`.tpl`) rendered at deploy with KV context (`get`, `getSecret`) — see [templates.md](./templates.md) |

Typical flow:

```text
edit workspace → maand build → maand deploy → health_check / job ops → gc
```

See [README.md](./README.md#typical-workflow) for the command sequence.

---

## Capability map (by concern)

| You need… | Maand provides… | Doc |
|-----------|-----------------|-----|
| Place jobs on labeled workers | Selectors + allocation auto-match | [concepts.md](./concepts.md), [resources-and-placement.md](./resources-and-placement.md) |
| Different CPU/memory per env | `bucket.jobs.prod.conf` + `job_config_selector` | [configuration.md](./configuration.md) |
| Ordered multi-job deploy | Command **demands** → `deployment_seq` waves | [jobs-and-dependencies.md](./jobs-and-dependencies.md) |
| Rolling restart without downtime | `update_parallel_count` + health between batches | [rolling-upgrade.md](./rolling-upgrade.md) |
| Skip redeploy when nothing changed | Content hashes + version promotion per allocation | [deploy.md](./deploy.md), `maand cat deployments` |
| Bootstrap secrets before templates | `pre_deploy` + `put_job_secret` / runtime API | [job-command.md](./job-command.md), [kv-variables.md](./kv-variables.md) |
| Custom canary or blue/green | `job_control` command + env `NEW_ALLOCATIONS` | [job-command.md](./job-command.md), [rolling-upgrade.md](./rolling-upgrade.md) |
| Single-writer migration across nodes | Runtime **semaphore** (`capacity=1`) | [job-command.md](./job-command.md#semaphores) |
| Drain one node for maintenance | `disabled.json` → build → deploy (stops, no restart) | [disabled.md](./disabled.md) |
| mTLS or app TLS on workers | Manifest `certs` + auto-rotation on build | [certs.md](./certs.md) |
| Debug “why didn’t deploy run?” | `--dry-run`, `maand cat deployments`, [deploy-debugging.md](./deploy-debugging.md) | [deploy-debugging.md](./deploy-debugging.md) |
| One-off operator script | `maand job_command` (event **`cli`**) | [job-command.md](./job-command.md) |

---

## Where maand is strong

1. **No agents** — workers only need SSH, `make`, `python3`, `rsync`, `bash`. Good fit when you do not want Nomad/K8s overhead.
2. **Declarative + incremental deploy** — content hashes and version tracking mean redeploys skip promoted allocations and resume after partial failure. Dry-run and `maand cat deployments` make rollout state visible. See [deploy.md](./deploy.md) and [deploy-debugging.md](./deploy-debugging.md).
3. **Multi-job orchestration** — dependency graph, ordered deploy waves, rolling restarts with health gates between batches. See [jobs-and-dependencies.md](./jobs-and-dependencies.md) and [rolling-upgrade.md](./rolling-upgrade.md).
4. **Extensibility without a custom agent** — job commands, runtime HTTP API (KV, demands, semaphores), and layered configuration support migrations, secret bootstrap, custom lifecycle, and CLI-triggered ops on the orchestrator host. See [job-command.md](./job-command.md) and [kv-variables.md](./kv-variables.md).
5. **Operational tooling built in** — maintenance disable (not delete), GC, dry-run, hash inspection, cert renewal on build, resource validation, and structured [deploy debugging](./deploy-debugging.md).
6. **Environment portability** — job manifests stay in git; bucket-level TOML and `job_config_selector` switch prod/staging reservations without forking job definitions. See [resources-and-placement.md](./resources-and-placement.md).

---

## Intentional limits

Maand is a **focused orchestrator**, not a general container or platform scheduler:

| Limit | Detail |
|-------|--------|
| **Single control point** | One bucket directory on one CLI host is the source of truth (not HA etcd/consul). |
| **SSH-centric** | All worker interaction is SSH/rsync; no native service mesh, CNI, or container runtime. |
| **Label-based placement** | Selectors + resource validation — not bin-packing, affinity rules, or dynamic scheduling beyond labels. |
| **Makefile lifecycle** | Default deploy uses `start`/`stop`/`restart` targets; you bring process supervision (systemd, containers, etc.) via Makefile or **`job_control`** commands. |
| **Hook runtimes** | Python3 and Bun on the **CLI host**; scripts reach workers via SSH helpers (Python) or your own wiring. Workers run what you deploy. |
| **In-process coordination** | Runtime API and semaphores exist only for the current maand CLI session — not a distributed lock service. |
| **Linux workers** | Docs assume Linux hosts with standard Unix tooling. |

---

## Summary

Maand can run a **multi-node cluster of stateful or stateless services** with:

- declarative jobs, selectors, and worker placement with resource bounds
- environment-specific reservations via bucket TOML
- ordered, rolling deploys with health checks and hash-based skip/resume
- TLS material, encrypted secrets, and a four-layer KV model
- rich lifecycle hooks (build, deploy, health, CLI) and manual ops
- maintenance disable and GC without losing workspace or worker data

It is best understood as **deploy orchestration plus a small job catalog** rather than a full platform like Kubernetes. For teams that want **dependency-aware rolling deploys, drain/disable, hook-driven ops, and inspectable rollout state** over a fixed set of SSH workers — without agents or a cluster control plane — maand is designed to cover that niche end to end.

---

## Further reading

### Concepts and workflow

- [concepts.md](./concepts.md) — workers, jobs, allocations, lifecycle
- [jobs-and-dependencies.md](./jobs-and-dependencies.md) — manifests, demands, versions, deploy waves
- [tutorials/getting-started.md](./tutorials/getting-started.md) — hands-on walkthrough
- [tutorials/day-2-operations.md](./tutorials/day-2-operations.md) — job control, health, disable, GC
- [tutorials/job-commands.md](./tutorials/job-commands.md) — Python/Bun hooks tutorial

### Configuration and data

- [configuration.md](./configuration.md) — `maand.conf`, `bucket.conf`, `bucket.jobs.conf`, `vars.toml`
- [resources-and-placement.md](./resources-and-placement.md) — memory/CPU, selectors, environment overrides
- [kv.md](./kv.md) · [kv-variables.md](./kv-variables.md) — namespaces, secrets, persistence, examples
- [templates.md](./templates.md) — `.tpl` rendering at deploy
- [certs.md](./certs.md) — TLS CA, job certs, auto-rotation

### Commands and operations

- [commands.md](./commands.md) — full CLI reference
- [build.md](./build.md) · [deploy.md](./deploy.md) · [health-check.md](./health-check.md)
- [job-command.md](./job-command.md) · [job.md](./job.md) · [run-command.md](./run-command.md) · [gc.md](./gc.md)
- [rolling-upgrade.md](./rolling-upgrade.md) · [disabled.md](./disabled.md) · [deploy-debugging.md](./deploy-debugging.md)
