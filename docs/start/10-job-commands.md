# 10. Job commands (hooks)

**Job commands** are Python or Bun scripts in `workspace/jobs/<job>/_modules/command_<name>.py` (or `.ts`). They run on the **CLI host**, not on workers — but each run is scoped to one **allocation** (worker IP + alloc id).

---

## Events (when hooks run)

| Event | Trigger |
|-------|---------|
| `post_build` | End of `maand build` |
| `pre_deploy` | Before rsync for a job wave |
| `post_deploy` | After rollout for a job wave |
| `job_control` | Instead of default `make start`/`restart`/… |
| `health_check` | From `maand health_check` (after manifest probes) |
| `after_allocation_started` | After a batch start/restart |
| `after_allocation_stopped` | After stop during reconcile |
| `cli` | Manual: `maand jobcommand <name> [job]` |

Declare in `manifest.json` under `commands`:

```json
{
  "commands": {
    "command_migrate": {
      "executed_on": ["pre_deploy"],
      "runtime": "python"
    }
  }
}
```

---

## Environment in scripts

Each invocation receives:

| Variable | Meaning |
|----------|---------|
| `ALLOCATION_ID` | Stable UUID |
| `ALLOCATION_IP` | Worker to target via SSH helpers |
| `JOB` | Job name |
| `EVENT` | Current event |
| `CURRENT_VERSION` / `NEW_VERSION` | Rollout context |

Python example:

```python
from maand import allocation_ip, run_ssh, put_job_secret

put_job_secret("token", "…").raise_for_status()
run_ssh(allocation_ip(), "systemctl is-active myapp").raise_for_status()
```

---

## Runtime HTTP API

While build/deploy/health_check/jobcommand runs, maand serves **localhost:8080**:

- GET/PUT/DELETE KV (scoped namespaces)
- Encrypted secrets
- **`acquire_semaphore`** / **`release_semaphore`** for cross-allocation locks
- **`list_command_demands`** — who depends on this job

Embedded **`maand.py`** / **`maand.ts`** wrap the API.

Reference: [job-command-api.md](../reference/job-command-api.md).

---

## Job commands vs Makefile vs run_command

| | Runs on | Use for |
|---|---------|---------|
| **Makefile** | Worker | Process start/stop, compose, systemd |
| **Job command** | CLI host | KV, secrets, migrations, custom rollout logic, SSH orchestration |
| **`maand run_command`** | Worker | Ad-hoc ops, `make logs`, debugging |

Rule of thumb: if it must mutate **catalog/KV** or coordinate **multiple allocations**, use a job command. If it only touches **local process files** on one host, use Makefile.

---

## Concurrency

```bash
maand jobcommand command_migrate api --concurrency 4
```

Parallel allocations run up to N at a time. Use semaphores when only one allocation should run DDL at a time.

Tutorial: [job-commands-tutorial.md](../guides/job-commands-tutorial.md)  
CLI: [job-command.md](../reference/cli/job-command.md).

---

## Next

[11 — Health, monitoring, and logs](./11-health-monitoring-and-logs.md).
