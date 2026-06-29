# Maand documentation

Maand is an **agentless** workload orchestrator. State lives in a local **bucket** (`maand.db`, KV, secrets, staged files). Workers are reached over **SSH** from the **CLI host**; job commands run on the host with **python3** and **bun**.

Official site: [maand.sh/latest](https://maand.sh/latest)

---

## How this documentation is organized

| Tier | Purpose | Start here |
|------|---------|------------|
| **Start** | Guided tour (Linux/K8s background → every feature) and first deploy | [start/README.md](./start/README.md) |
| **Guides** | Task-oriented how-tos | [guides/README.md](./guides/README.md) |
| **Reference** | Schemas, CLI, KV, configuration | [reference/README.md](./reference/README.md) |

Each topic has **one canonical page**. Other files summarize and link — they do not repeat full schemas.

---

## Typical workflow

```text
maand init → edit workspace → maand build → maand deploy → health_check / job ops → gc
```

---

## Bucket layout

```text
./                          # bucket root (run all maand commands here)
├── maand.conf              # SSH, certs, job_config_selector, log_format
├── data/maand.db
├── workspace/
│   ├── workers.json
│   ├── disabled.json       # optional
│   ├── bucket.conf
│   ├── bucket.jobs.conf    # optional; or bucket.jobs.<env>.conf
│   └── jobs/<job>/
│       ├── manifest.json
│       ├── Makefile        # or Makefile.tpl
│       ├── _modules/
│       ├── _prometheus/    # optional
│       └── *.tpl
├── secrets/
├── tmp/
└── logs/                   # structured command logs; logs/runs/<run_id>/
```

---

## Reader paths

| Goal | Path |
|------|------|
| Learn maand from scratch | [start/README.md](./start/README.md) (chapters 1–12) → [quickstart](./start/quickstart.md) |
| First deploy only | [quickstart](./start/quickstart.md) |
| Glossary | [concepts](./start/concepts.md) |
| Memory / CPU / environments | [resources-and-placement](./reference/resources-and-placement.md) → [configuration](./reference/configuration.md) |
| Rolling cluster upgrade | [rolling-deploy](./guides/rolling-deploy.md) → [manifest](./reference/manifest.md) |
| Multi-job deploy order | [deployment-sequence](./reference/deployment-sequence.md) → [deploy](./reference/cli/deploy.md) |
| Deploy failed or skipped | [debugging-deploy](./guides/debugging-deploy.md) → [logging](./reference/observability/logging.md) |
| Push config without restart | [deploy — applying changes](./reference/cli/deploy.md#applying-changes-on-workers) — **`restart_policy: reload`**, **`restart_globs`**, **`--sync-only`** |
| Disable / drain | [disable-and-drain](./guides/disable-and-drain.md) |
| Job hooks (Python/Bun) | [job-commands-tutorial](./guides/job-commands-tutorial.md) → [job-command](./reference/cli/job-command.md) |
| Prometheus integration | [prometheus](./guides/prometheus.md) |
| Compare to other tools | [comparison-orchestrators](./start/comparison-orchestrators.md) |

---

## Inspect state

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
maand cat deployments
maand cat certs
maand cat kv get maand/job/<job> version
maand logs show --job <job> --format human
```

---

## Local integration tests

See [assets/README.md](../assets/README.md), then:

```bash
go test -tags=integration ./tests/integration/... -v -timeout 25m
```

Not run in CI.
