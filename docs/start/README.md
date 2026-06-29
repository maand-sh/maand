# Learn maand

This track is for operators who already know **Linux** (SSH, users, systemd or Docker, Make) and may have used **Ansible**, **Kubernetes**, **Nomad**, or similar tools. It introduces maand **one idea at a time**, then links to reference pages for flags and schemas.

**You do not need to read everything before your first deploy.** Two paths:

| Path | Best if you… |
|------|----------------|
| **Guided tour** (chapters below) | Want to understand *why* before *how* |
| **Hands-on first** | Prefer typing commands → [quickstart.md](./quickstart.md), then come back here |

After the tour, use [concepts.md](./concepts.md) as a consolidated glossary and [overview.md](./overview.md) as a feature checklist.

---

## Chapters

Read in order. Each chapter is short; skip ahead only if the topic is already familiar.

| # | Chapter | You will learn |
|---|---------|----------------|
| 1 | [Introduction](./01-introduction.md) | What maand is, what runs where, typical workflow |
| 2 | [From your background](./02-from-your-background.md) | Map Linux / Ansible / K8s / Nomad ideas to maand |
| 3 | [Bucket and workspace](./03-bucket-and-workspace.md) | `maand init`, directory layout, what you edit vs what maand generates |
| 4 | [Workers](./04-workers.md) | `workers.json`, SSH, labels, capacity, `worker_facts` |
| 5 | [Jobs and lifecycle](./05-jobs-and-lifecycle.md) | `manifest.json`, Makefile, start/stop/restart, containers and logs |
| 6 | [Allocations and placement](./06-allocations-and-placement.md) | Job × worker matching, selectors, disable without delete |
| 7 | [Build](./07-build.md) | Catalog, certs, validation — still no deploy to workers |
| 8 | [Deploy](./08-deploy.md) | Rsync, rolling batches, restart vs reload, dry-run |
| 9 | [Configuration, KV, and templates](./09-configuration-kv-and-templates.md) | Secrets, per-env CPU/memory, `.tpl` files |
| 10 | [Job commands (hooks)](./10-job-commands.md) | Python/Bun on the CLI host, runtime API, when to use hooks |
| 11 | [Health, monitoring, and logs](./11-health-monitoring-and-logs.md) | Probes, Prometheus folder, maand logs vs app logs |
| 12 | [Day-2 operations](./12-day-2-operations.md) | `maand job`, `run_command`, GC, drain, debugging |

**Hands-on capstone:** [quickstart.md](./quickstart.md) — init through first deploy.

**Optional:** [comparison-orchestrators.md](./comparison-orchestrators.md) — deeper comparison to Kubernetes and Nomad.

---

## Where to go next

| Goal | Document |
|------|----------|
| Full CLI list | [commands.md](../reference/cli/commands.md) |
| Manifest fields | [manifest.md](../reference/manifest.md) |
| Rolling upgrades | [rolling-deploy.md](../guides/rolling-deploy.md) |
| Deploy failures | [debugging-deploy.md](../guides/debugging-deploy.md) |
| Hooks tutorial | [job-commands-tutorial.md](../guides/job-commands-tutorial.md) |
| Doc index | [README.md](../README.md) |
