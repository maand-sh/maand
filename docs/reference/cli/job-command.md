# Job commands

**Job commands** are Python or Bun scripts under `workspace/jobs/<job>/_modules/` that maand runs on the **CLI host**. Each invocation is scoped to one **allocation** (job on a worker).

| Doc | Contents |
|-----|----------|
| [manifest.md](../manifest.md) | `commands` block in `manifest.json` |
| [job-command-api.md](../job-command-api.md) | HTTP API, env vars, Python/Bun helpers |
| [guides/job-commands-tutorial.md](../../guides/job-commands-tutorial.md) | Hands-on walkthrough |

---

## When to use job commands

| Need | Prefer |
|------|--------|
| Static config in git | `vars.toml`, `bucket.jobs*.conf` |
| Process lifecycle on workers | Makefile + `runner.py` (default deploy) |
| Custom full rollout | **`job_control`** command |
| Batched start/restart + per-batch hooks | **`after_allocation_started`** + parallel counts — [guides/rolling-deploy.md](../../guides/rolling-deploy.md) |
| Drain on stop | **`after_allocation_stopped`** |
| Secrets before `.tpl` render | **`pre_deploy`** + `put_job_secret` |
| Post-rollout smoke test | **`post_deploy`** |
| Probes | manifest `health_check` and/or **`health_check`** command — [health-check.md](health-check.md) |
| Operator one-off | **`cli`** + `maand job_command` |

Scripts run on the **CLI host**, not on workers by default. Use Python **`run_ssh`** / **`run_runner_target`** to reach workers.

**Disabled allocations** still receive invocations (`DISABLED=1`). See [guides/disable-and-drain.md](../../guides/disable-and-drain.md).

---

## CLI (ad-hoc)

```bash
maand job_command <job> <command_name> [--concurrency N] [--verbose]
```

Requires **`cli`** in manifest **`executed_on`**. KV commits on success.

---

## Command events (`executed_on`)

| Event | Triggered by | Summary |
|-------|----------------|---------|
| **`post_build`** | **`maand build`** | After catalog commit; **build fails** on error; runs in `deployment_seq` order |
| **`pre_deploy`** | **`maand deploy`** | Before rsync; secrets/vars for templates; failure skips job for this deploy |
| **`post_deploy`** | **`maand deploy`** | After successful rollout; failure fails that job |
| **`job_control`** | **`maand deploy`** | Replaces Makefile start/restart/stop entirely |
| **`health_check`** | **`maand health_check`**, deploy | KV read-only; mutually exclusive with manifest probes |
| **`cli`** | **`maand job_command`** | Operator-triggered only |
| **`after_allocation_started`** | **`maand deploy`** | After each batch start/restart, before health gate |
| **`after_allocation_stopped`** | **`maand deploy`** | After each allocation stop during reconcile |

### Batch env (allocation hooks)

See [job-command-api.md](../job-command-api.md#environment-variables-job-command-scripts) for per-allocation env (`ALLOCATION_ID`, `ALLOCATION_IP`, **`ALLOCATION_INDEX`**, `CURRENT_VERSION`, `NEW_VERSION`, …) and batch hooks (`BATCH_*`, `DEPLOY_PHASE`, `DEPLOY_ORDER`).

**`pre_deploy`** may override **`deploy_order`** with **`put_deploy_order()`** for one deploy; build resets it on the next **`maand build`**. See [job-command-api.md](../job-command-api.md).

### Demands

Optional upstream dependency for **`deployment_seq`**. See [deployment-sequence.md](../deployment-sequence.md).

---

## Event behavior (detail)

### `post_build`

Validate artifacts, codegen, seed **`vars/job/*`**, cross-job checks. Runs in **`deployment_seq`** order. Any failure fails **`maand build`**.

### `pre_deploy`

Fetch secrets, write **`secrets/job/*`** and **`vars/job/*`** for `.tpl` on this deploy, external prerequisite checks. Override rollout order with **`put_deploy_order("10.0.0.2,10.0.0.1")`**. Failure: job not staged this deploy.

### `post_deploy`

Smoke tests, cache warming, service registration. Runs after rollout (and health gate when configured).

### `job_control`

Implement custom rollout with `NEW_ALLOCATIONS` / `UPDATED_ALLOCATIONS` and **`run_runner_target`** / **`run_ssh`**. Still followed by health check and **`post_deploy`**.

### `health_check`

See [health-check.md](health-check.md). KV writes rejected. **`maand health_check --update-hash`** can mark failed allocations for redeploy.

### `after_allocation_started` / `after_allocation_stopped`

Per-allocation hooks during batched rollout or stop reconcile. Use for cluster join, leader election, drain. See [guides/rolling-deploy.md](../../guides/rolling-deploy.md).

### `cli`

Test commands before wiring deploy events; operator maintenance. Commits KV on success.

---

## File layout

| Rule | Detail |
|------|--------|
| Command name | Prefix **`command_`** |
| Script | One of `command_<name>.py`, `.ts`, `.js` under `_modules/` |

```json
{
  "commands": {
    "command_init": {
      "executed_on": ["pre_deploy", "cli"],
      "demands": { "job": "", "command": "", "config": {} }
    }
  }
}
```

---

## Default deploy without `job_control`

Deploy runs **`pre_deploy`**, Makefile lifecycle (batched), **`after_allocation_started`** (if registered), health checks, **`post_deploy`**.

Full pipeline: [deploy.md](deploy.md#default-deploy-path-makefile--runner).

---

## Example: secret bootstrap + template

**`command_bootstrap.py`** (`pre_deploy`):

```python
from maand import put_job_secret, put_job_variable

put_job_secret("db_password", fetch_from_vault()).raise_for_status()
put_job_variable("db_host", "db.internal").raise_for_status()
```

**`config.toml.tpl`**:

```toml
db_host = "{{ get "vars/job/api" "db_host" }}"
db_password = "{{ getSecret "secrets/job/api" "db_password" }}"
```

---

## Checklist

1. Add `command_<name>.py` (or `.ts`) under `_modules/`.
2. Register in **`manifest.json`** with **`executed_on`** and optional **`demands`**.
3. **`maand build`**
4. Test: **`maand job_command <job> command_<name> --verbose`** (if `cli` listed)
5. Wire deploy events as needed.

---

## CLI errors

| Error | Meaning |
|-------|---------|
| `NotFoundError` | Command not allowed for this event on this job |
| `RunError` | One or more allocations failed |
| `WorkerFailure` | Script exited non-zero or SSH failure |
| File not found | Missing or duplicate `.py`/`.ts`/`.js` implementation |

HTTP API errors: [job-command-api.md](../job-command-api.md#http-api-errors).

---

## Who runs what

```text
                    post_build   pre_deploy   deploy roll   post_deploy   health_check   cli
maand build              ✓            —            —            —              —         —
maand deploy             —            ✓            ✓*           ✓              ✓*        —
maand health_check       —            —            —            —              ✓         —
maand job_command        —            —            —            —              —         ✓

* deploy: health_check after restart/job_control; roll = job_control OR Makefile path
                         (+ after_allocation_started/stopped when registered)
```

---

## Related

- [job-command-api.md](../job-command-api.md)
- [cli/deploy.md](deploy.md) · [cli/build.md](build.md)
- [kv/namespaces.md](../kv/namespaces.md)
