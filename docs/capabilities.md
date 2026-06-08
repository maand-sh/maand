# Capabilities overview

Maand is an **agentless workload orchestrator** for Linux clusters — more than a simple deploy script, but narrower in scope than Kubernetes or Nomad.

A **CLI host** (laptop, CI runner, or bastion) holds a local **bucket**: SQLite catalog, encrypted KV, secrets, and staged files. **Workers** are ordinary Linux hosts reached over **SSH** and **rsync**. Nothing maand-specific is installed on workers except what you deploy (`runner.py`, job files, Makefiles).

That design fits **small-to-medium fleets** where you want declarative jobs, rolling deploys, and ops hooks without running a separate control-plane cluster.

Related: [concepts.md](./concepts.md) · [commands.md](./commands.md) · [configuration.md](./configuration.md) · [getting started](./tutorials/getting-started.md)

---

## Core capabilities

| Area | What maand can do |
|------|-------------------|
| **Catalog & placement** | Declare workers (`workers.json`) with labels, tags, CPU/memory; declare jobs with selectors; auto-create **allocations** (job × worker) by label matching |
| **Build** | Reconcile workspace → `maand.db` + KV; validate resources, ports, job dependencies; generate TLS certs; run `post_build` hooks |
| **Deploy** | Rsync to `/opt/worker/<bucket_id>/`; Makefile start/stop/restart (or custom `job_control`); **hash-based skip** for unchanged jobs; **partial deploy resume**; `--dry-run`, `--force`, `--jobs` filters |
| **Rolling upgrades** | `update_parallel_count` for batched restarts; semver **`version`** tracking (`CURRENT_VERSION` / `NEW_VERSION` in Makefiles); deployment **waves** via `deployment_seq` from job **demands** |
| **Dependencies** | Cross-job command dependencies with version constraints; circular demand detection at build time |
| **Hooks** | Python/Bun **job commands** on the CLI host for `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, and ad-hoc `cli` |
| **Health** | Built-in TCP/HTTP/SSH probes, or custom health commands; `--wait`, `--update-hash` to trigger redeploy |
| **Day-2 ops** | `maand job start\|stop\|restart\|run\|status`; `maand run_command` for ad-hoc SSH; disable/drain via `disabled.json` without deleting workspace |
| **State & secrets** | Encrypted KV (`secrets/job/...`), per-allocation vars, `maand cat` for inspection; GC for removed allocations and old KV history |
| **Templates** | Go templates (`.tpl`) rendered at deploy time with KV context |

Typical flow:

```text
edit workspace → maand build → maand deploy → health_check / job ops → gc
```

See [README.md](./README.md#typical-workflow) for the command sequence.

---

## Where maand is strong

1. **No agents** — workers only need SSH, `make`, `python3`, `rsync`, `bash`. Good fit when you do not want Nomad/K8s overhead.
2. **Declarative + incremental deploy** — content hashes and version tracking mean redeploys skip promoted allocations and resume after partial failure. See [deploy.md](./deploy.md) and [deploy-debugging.md](./deploy-debugging.md).
3. **Multi-job orchestration** — dependency graph, ordered deploy waves, rolling restarts with health gates between batches. See [jobs-and-dependencies.md](./jobs-and-dependencies.md) and [rolling-upgrade.md](./rolling-upgrade.md).
4. **Extensibility without a custom agent** — job commands, runtime HTTP API, and KV support migrations, secret bootstrap, custom lifecycle, and CLI-triggered ops on the orchestrator host. See [job-command.md](./job-command.md).
5. **Operational tooling built in** — disable/drain, GC, dry-run, hash inspection (`maand cat hashes`), and [deploy debugging](./deploy-debugging.md).

---

## Intentional limits

Maand is a **focused orchestrator**, not a general container or platform scheduler:

| Limit | Detail |
|-------|--------|
| **Single control point** | One bucket directory on one CLI host is the source of truth (not HA etcd/consul). |
| **SSH-centric** | All worker interaction is SSH/rsync; no native service mesh, CNI, or container runtime. |
| **Label-based placement** | Selectors + resource validation — not bin-packing, affinity rules, or dynamic scheduling beyond labels. |
| **Makefile lifecycle** | Default deploy uses `start`/`stop`/`restart` targets; you bring process supervision (systemd, containers, etc.) via Makefile or `job_control` commands. |
| **Hook runtimes** | Python3 and Bun on the CLI host; workers run what you deploy. |
| **Linux workers** | Docs assume Linux hosts with standard Unix tooling. |

---

## Summary

Maand can run a **multi-node cluster of stateful or stateless services** with:

- declarative jobs and worker placement
- ordered, rolling deploys with health checks
- secrets, certs, and a KV layer
- rich lifecycle hooks and manual ops

It is best understood as **deploy orchestration plus a small job catalog** rather than a full platform like Kubernetes. For teams that want **dependency-aware rolling deploys, drain/disable, and hook-driven ops** over a fixed set of SSH workers — without agents or a cluster control plane — maand is designed to cover that niche end to end.

---

## Further reading

- [concepts.md](./concepts.md) — workers, jobs, allocations, lifecycle
- [jobs-and-dependencies.md](./jobs-and-dependencies.md) — manifests, demands, versions, deploy waves
- [kv.md](./kv.md) · [templates.md](./templates.md) — config data and deploy rendering
- [tutorials/getting-started.md](./tutorials/getting-started.md) — hands-on walkthrough
- [tutorials/day-2-operations.md](./tutorials/day-2-operations.md) — job control, health, disable, GC
