# Capabilities overview

Maand is an **agentless workload orchestrator** for Linux clusters — more than a simple deploy script, but narrower in scope than Kubernetes or Nomad.

A **CLI host** (laptop, CI runner, or bastion) holds a local **bucket**: SQLite catalog, encrypted KV, secrets, and staged files. **Workers** are ordinary Linux hosts reached over **SSH** and **rsync**. Nothing maand-specific is installed on workers except what you deploy (`runner.py`, job files, Makefiles).

That design fits **small-to-medium fleets** where you want declarative jobs, rolling deploys, and ops hooks without running a separate control-plane cluster.

Related: [concepts.md](concepts.md) · [commands.md](../reference/cli/commands.md) · [configuration.md](../reference/configuration.md) · [getting started](quickstart.md)

---

## Core capabilities

| Area | What maand can do |
|------|-------------------|
| **Catalog & placement** | Declare workers (`workers.json`) with labels, tags, CPU/memory; declare jobs with **selectors** (label matching); auto-create **allocations** (job × worker). Three-layer **resource model**: manifest min/max, `bucket.jobs*.conf` reservations, worker capacity validation — see [resources-and-placement.md](../reference/resources-and-placement.md) |
| **Environment overrides** | `job_config_selector` in `maand.conf` picks `bucket.jobs.<env>.conf` for per-environment memory/CPU without changing manifests — see [configuration.md](../reference/configuration.md) |
| **Build** | Reconcile workspace → `maand.db` + KV; validate resources, ports, job dependencies; generate and **auto-rotate TLS** certs; sync `disabled.json`; run **`post_build`** hooks in `deployment_seq` order — see [build.md](../reference/cli/build.md), [certs.md](../reference/certs.md) |
| **Deploy** | Rsync to `/opt/worker/<bucket_id>/`; Makefile start/stop/restart (or custom **`job_control`**); **hash-based skip** for unchanged jobs; **partial deploy resume**; **`--dry-run`**, **`--force`**, **`--jobs`** filters; ordered **waves** via `deployment_seq` — see [deploy.md](../reference/cli/deploy.md), [deploy-debugging.md](../guides/debugging-deploy.md) |
| **Rolling upgrades** | `deploy_parallel_count` / `update_parallel_count`, health gates, `deploy_order`; semver version tracking — [guides/rolling-deploy.md](../guides/rolling-deploy.md) |
| **Dependencies** | Cross-job **command demands** with version constraints; build computes **`deployment_seq`** and detects circular demands — see [deployment-sequence.md](../reference/deployment-sequence.md) |
| **Job commands & runtime API** | Python/Bun hooks on the CLI host: `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, `after_allocation_started`, `after_allocation_stopped`, **`cli`** — [reference/cli/job-command.md](../reference/cli/job-command.md) |
| **Health** | Built-in TCP/HTTP/SSH manifest probes and/or custom **`health_check`** commands (probes first); **`--wait`**, **`--update-hash`** to trigger redeploy — see [health-check.md](../reference/cli/health-check.md) |
| **Prometheus** | Per-job optional **`_prometheus/`** (scrape, alerts, runbooks, dashboards); build validates and stores in `job_files`; scrape configs go to KV — see [prometheus.md](../guides/prometheus.md) |
| **Maintenance (disable)** | **`disabled.json`** drains workers, jobs, or single allocations **without** removing workspace or catalog rows. Disabled allocations still get build/KV/certs and deploy staging; maand **never starts** them — see [disabled.md](../guides/disable-and-drain.md) |
| **Day-2 ops** | `maand job start\|stop\|restart\|run\|status`; `maand run_command` for ad-hoc SSH; soft-remove via workspace edits + **GC** for purged rows and disk — see [job.md](../reference/cli/job.md), [gc.md](../reference/cli/gc.md), [tutorials/day-2-operations.md](../guides/day-2-ops.md) |
| **State & secrets** | Layered KV: global (`vars/bucket`), worker, job (`vars.toml` + hooks), allocation (certs, peers, version). Encrypted **`secrets/job/*`**. Inspect with **`maand cat`** — see [KV persistence](../reference/kv/persistence.md), [KV namespaces](../reference/kv/namespaces.md) |
| **TLS** | Bucket CA at init; per-job certs declared in manifest; build-time renewal via `certs_ttl` / `certs_renewal_buffer`; staged to workers on deploy — see [certs.md](../reference/certs.md) |
| **Templates** | Go templates (`.tpl`) rendered at deploy with KV context (`get`, `getSecret`) — see [templates.md](../reference/templates.md) |

Typical flow:

```text
edit workspace → maand build → maand deploy → health_check / job ops → gc
```

See [README.md](../README.md#typical-workflow) for the command sequence.

---

## Capability map (by concern)

| You need… | Maand provides… | Doc |
|-----------|-----------------|-----|
| Place jobs on labeled workers | Selectors + allocation auto-match | [concepts.md](concepts.md), [resources-and-placement.md](../reference/resources-and-placement.md) |
| Different CPU/memory per env | `bucket.jobs.prod.conf` + `job_config_selector` | [configuration.md](../reference/configuration.md) |
| Ordered multi-job deploy | Command **demands** → `deployment_seq` waves | [deployment-sequence.md](../reference/deployment-sequence.md) |
| Rolling restart without downtime | `update_parallel_count` + health between batches | [rolling-deploy](../guides/rolling-deploy.md) |
| Skip redeploy when nothing changed | Content hashes + version promotion per allocation | [deploy.md](../reference/cli/deploy.md), `maand cat deployments` |
| Bootstrap secrets before templates | `pre_deploy` + `put_job_secret` / runtime API | [job-command-api.md](../reference/job-command-api.md), [KV namespaces](../reference/kv/namespaces.md) |
| Custom canary or blue/green | `job_control` command + env `NEW_ALLOCATIONS` | [job-command-api.md](../reference/job-command-api.md), [rolling-deploy](../guides/rolling-deploy.md) |
| Single-writer migration across nodes | Runtime **semaphore** (`capacity=1`) | [job-command-api.md](../reference/job-command-api.md#semaphores) |
| Drain one node for maintenance | `disabled.json` → build → deploy (stops, no restart) | [disabled.md](../guides/disable-and-drain.md) |
| mTLS or app TLS on workers | Manifest `certs` + auto-rotation on build | [certs.md](../reference/certs.md) |
| Inspect TLS expiry | `maand cat certs` | [certs.md](../reference/certs.md#inspecting-certificates-maand-cat-certs) |
| Debug “why didn’t deploy run?” | `--dry-run`, `maand cat deployments`, [deploy-debugging.md](../guides/debugging-deploy.md) | [deploy-debugging.md](../guides/debugging-deploy.md) |
| One-off operator script | `maand job_command` (event **`cli`**) | [job-command-api.md](../reference/job-command-api.md) |

---

## Where maand is strong

1. **No agents** — workers only need SSH, `make`, `python3`, `rsync`, `bash`. Good fit when you do not want Nomad/K8s overhead.
2. **Declarative + incremental deploy** — content hashes and version tracking mean redeploys skip promoted allocations and resume after partial failure. Dry-run and `maand cat deployments` make rollout state visible. See [deploy.md](../reference/cli/deploy.md) and [deploy-debugging.md](../guides/debugging-deploy.md).
3. **Multi-job orchestration** — dependency graph, ordered deploy waves, rolling restarts with health gates between batches. See [deployment-sequence.md](../reference/deployment-sequence.md) and [rolling-deploy](../guides/rolling-deploy.md).
4. **Extensibility without a custom agent** — job commands, runtime HTTP API (KV, demands, semaphores), and layered configuration support migrations, secret bootstrap, custom lifecycle, and CLI-triggered ops on the orchestrator host. See [job-command-api.md](../reference/job-command-api.md) and [KV namespaces](../reference/kv/namespaces.md).
5. **Operational tooling built in** — maintenance disable (not delete), GC, dry-run, hash inspection, cert renewal on build, resource validation, and structured [deploy debugging](../guides/debugging-deploy.md).
6. **Environment portability** — job manifests stay in git; bucket-level TOML and `job_config_selector` switch prod/staging reservations without forking job definitions. See [resources-and-placement.md](../reference/resources-and-placement.md).

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

- [concepts.md](./concepts.md) · [quickstart.md](./quickstart.md)
- [Documentation index](../README.md)
