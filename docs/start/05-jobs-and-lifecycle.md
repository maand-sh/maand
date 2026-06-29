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

During lifecycle targets, the Makefile receives:

- **`CURRENT_VERSION`** — last promoted version on this allocation
- **`NEW_VERSION`** — target from manifest after build

Use them for migrations or version-specific scripts.

You can add other targets (`test`, `backup`, …) for `maand job run` or `maand run_command` — maand only invokes the lifecycle set above during deploy.

---

## Docker Compose or systemd behind Make

Maand does not start containers or units itself. Your **Makefile** wraps them:

**Compose:**

```makefile
COMPOSE := docker compose -f docker-compose.yml

start:
	$(COMPOSE) up -d --remove-orphans

stop:
	$(COMPOSE) down

restart: stop start
```

**systemd:**

```makefile
UNIT := api.service

start:
	sudo systemctl start $(UNIT)

stop:
	sudo systemctl stop $(UNIT)

restart: stop start
```

Deploy runs `make start` / `restart` / `reload` over SSH on each allocation.

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

Those are created on workers by your Makefile or application at runtime (and are excluded from rsync on upgrade).

---

## Next

[06 — Allocations and placement](./06-allocations-and-placement.md) — how maand picks which worker runs which job.
