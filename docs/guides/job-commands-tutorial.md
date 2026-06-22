# Tutorial: Job commands

**Job commands** are Python or Bun scripts under `workspace/jobs/<job>/_modules/`. They run on the **maand CLI host** (not on workers) once per **active allocation**, with access to maand’s **runtime HTTP API** and **KV store**.

Use them for migrations, secret handling, config generation, and deploy hooks — not for long-running services (those use the job **Makefile** on workers).

Prerequisites: [quickstart](../start/quickstart.md) · Deep dive: [job-command-api.md](../reference/job-command-api.md)

---

## Step 1 — Declare a command

Edit **`workspace/jobs/api/manifest.json`**:

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "commands": {
    "command_migrate": {
      "executed_on": ["cli", "pre_deploy"]
    },
    "command_health": {
      "executed_on": ["health_check"]
    }
  }
}
```

Rules enforced at build:

- Command names must start with **`command_`**
- **`executed_on`** must use allowed events: `post_build`, `pre_deploy`, `post_deploy`, `job_control`, `health_check`, `cli`
- A job with **`executed_on`: `["health_check"]`** cannot also define manifest **`health_check`** probes (build fails)
- Matching script must exist: `_modules/command_<name>.py` (or `.ts` / `.js`)

---

## Step 2 — Add the script

**`workspace/jobs/api/_modules/command_migrate.py`**:

```python
#!/usr/bin/env python3
import maand

def main():
    job = maand.job()
    worker = maand.worker()
    print(f"migrate api on {worker['worker_ip']} alloc={job['allocation_id']}")

    # Example: read KV
    name = maand.kv.get("maand/job/api", "name")
    print(f"job name from KV: {name}")

if __name__ == "__main__":
    main()
```

Maand injects **`maand.py`** (or **`maand.ts`**) into each allocation workspace under `tmp/` during execution.

Optional: job-local venv — place **`_modules/.venv/`** and maand uses that Python instead of system `python3`.

For Bun, use **`command_foo.ts`** and ensure **`bun`** is on the CLI host.

---

## Step 3 — Build and run from CLI

```bash
maand build
maand job_command api command_migrate --verbose
maand job_command api command_migrate --concurrency 2
```

Before running, maand checks:

- **`python3`** (or venv) for `.py` commands
- **`bun`** for `.ts`/`.js` commands

---

## Step 4 — Wire into deploy

With **`pre_deploy`** in `executed_on`, **`maand deploy`** runs the command automatically before staging that job’s wave:

```bash
maand deploy --jobs api
```

Other useful events:

| Event | When it runs |
|-------|----------------|
| `post_build` | End of `maand build` (build fails if hook fails) |
| `pre_deploy` | Before rsync for that job |
| `post_deploy` | After successful rollout |
| `job_control` | Instead of default `make start/stop/restart` |
| `health_check` | `maand health_check` and after deploy restart |
| `cli` | `maand job_command` only |

---

## Step 5 — Dependencies and version

If **`api`** depends on **`database`**, both jobs need explicit **`version`** fields:

```json
{
  "version": "2.0.0",
  "commands": {
    "command_migrate": {
      "executed_on": ["pre_deploy"],
      "demands": {
        "job": "database",
        "command": "command_schema",
        "config": {
          "min_version": "1.0.0"
        }
      }
    }
  }
}
```

**`maand build`** verifies demand job/command exist, both sides declare **`version`**, and upstream version satisfies **`min_version`** / **`max_version`**. Build also sets **`deployment_seq`** so **`database`** deploys before **`api`**.

Full reference: [manifest.md](../reference/manifest.md#version) · deploy upgrade env: [deploy.md](../reference/cli/deploy.md#allocation-version-tracking)

---

## Step 6 — Health check command

```bash
maand health_check --jobs api --wait --verbose
```

Or trigger after manual restart:

```bash
maand job restart api --health_check
```

---

## KV and secrets from commands

Commands read/write KV via the runtime API (`maand.kv.get`, `maand.kv.put`, …). Build populates namespaces such as:

- `maand/worker/<ip>` — worker metadata
- `maand/job/<job>` — job metadata
- `maand/job/<job>/worker/<ip>` — per-allocation certs and vars

Template files (`.tpl`) rendered at deploy use the same data. See [job-command-api.md](../reference/job-command-api.md) (runtime API) and [KV persistence](../reference/kv/persistence.md) / [templates.md](../reference/templates.md).

---

## Quick reference

```bash
maand build                                    # validates commands + scripts
maand job_command <job> <command> [--verbose]  # event: cli
maand deploy                                   # pre/post_deploy, job_control
maand health_check --jobs <job>
maand cat job_commands                         # list registered commands
```

Next: [day-2-ops.md](./day-2-ops.md) · [concepts.md](../start/concepts.md)
