# 5. Jobs and lifecycle

A **job** is a deployable unit: a directory under **`workspace/jobs/<name>/`** with a manifest, Makefile, and supporting files.

---

## Minimum layout

```text
workspace/jobs/api/
├── manifest.json    # version, selectors, resources, commands, …
├── Makefile         # start, stop, restart (required for default deploy)
└── …                # configs, scripts, docker-compose.yml.tpl, etc.
```

Scaffold a new job:

```bash
maand job create api --selectors worker
```

---

## Makefile contract

Default deploy expects these targets on the **worker** (after rsync):

| Target | When maand calls it |
|--------|---------------------|
| **`start`** | First deploy to a new allocation |
| **`stop`** | Reconcile removed/disabled allocation |
| **`restart`** | Upgrade when `restart_policy: always`, or when `restart_globs` match |
| **`reload`** | Upgrade when `restart_policy: reload` and globs don't force restart |
| **`status`** | `maand job status` (optional but useful) |
| **`logs`** | Not called by maand — for operators via `run_command` (below) |

During lifecycle targets, the Makefile receives:

- **`CURRENT_VERSION`** — last promoted version on this allocation
- **`NEW_VERSION`** — target from manifest after build

Use them for migrations or version-specific scripts.

---

## Docker Compose or systemd behind Make

Maand does not start containers itself. Typical patterns:

**Compose:**

```makefile
COMPOSE := docker compose -f docker-compose.yml

start:
	$(COMPOSE) up -d --remove-orphans

stop:
	$(COMPOSE) down

restart: stop start

logs:
	$(COMPOSE) logs --tail=100 --no-color
```

**systemd:**

```makefile
UNIT := api.service

start:
	sudo systemctl start $(UNIT)

stop:
	sudo systemctl stop $(UNIT)

logs:
	journalctl -u $(UNIT) -n 100 --no-pager
```

Deploy runs `make start` / `restart` / `reload` over SSH. Container stdout goes to Docker or journald unless you also write files under `./logs/`.

---

## Application logs (snapshot)

Add a **`logs`** target that prints recent lines and **exits** (no `-f` follow). Fetch via `run_command`:

```bash
maand run_command "make -s -C jobs/api logs" --workers 10.0.0.1
maand run_command "make -s -C jobs/api logs LOG_TAIL=500" --workers 10.0.0.1
```

Working directory on the worker is `/opt/worker/<bucket_id>/`, so `jobs/api` resolves correctly.

For **live follow**, use `run_command` with a follow command or SSH directly — `run_command` uses a remote timeout suited for short commands. See [11-health-monitoring-and-logs.md](./11-health-monitoring-and-logs.md).

---

## Manifest essentials

```json
{
  "version": "1.2.0",
  "selectors": ["worker", "web"],
  "update_parallel_count": 2,
  "restart_policy": "always",
  "resources": {
    "memory": { "min": "256 mb", "max": "512 mb" },
    "cpu": { "min": "200 mhz", "max": "500 mhz" },
    "ports": { "http": {} }
  }
}
```

| Field | Meaning |
|-------|---------|
| `version` | Release id (semver-style); drives rollout when it changes |
| `selectors` | Worker labels required for placement (chapter 6) |
| `update_parallel_count` | Rolling batch size for restart/reload during deploy |
| `restart_policy` | `always` / `reload` / `never` — see [chapter 8](./08-deploy.md) |
| `resources` | Bounds validated against worker capacity; ports from pool or fixed |

Full schema: [manifest.md](../reference/manifest.md).

---

## What not to commit in workspace

Build rejects runtime dirs inside the job workspace:

- `data/`, `logs/`, `bin/`

Those are created on workers by your Makefile or application.

---

## Next

[06 — Allocations and placement](./06-allocations-and-placement.md) — how maand picks which worker runs which job.
