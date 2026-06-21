# Job commands

**Job commands** are Python or Bun scripts under `workspace/jobs/<job>/_modules/` that maand runs on the **CLI host** (the machine where you run `maand build`, `maand deploy`, and `maand job_command`). Each invocation is scoped to one **allocation** (job on a worker). Scripts read and write configuration through a **runtime HTTP API** backed by the in-memory **KV store** loaded from `maand.db`.

Commands are declared in **`manifest.json`** with an **`executed_on`** list that controls when maand may run them.

Related: [kv-variables.md](./kv-variables.md) · [jobs-and-dependencies.md](./jobs-and-dependencies.md) · [health-check.md](./health-check.md) · [tutorials/job-commands.md](./tutorials/job-commands.md)

---

## When to use job commands

Use job commands when you need **orchestrator-side logic** tied to build, deploy, health, or ad-hoc ops — especially when that logic must run **once per allocation** and may need **KV**, **secrets**, or **cross-allocation coordination**.

| Need | Prefer | Why |
|------|--------|-----|
| Static config checked into git | `vars.toml`, `vars/bucket/*.toml` | No code; rendered into templates at deploy |
| Start/stop/restart a process on workers | Makefile + `runner.py` (default deploy path) | Simple lifecycle; no custom script |
| Custom rollout (canary, blue/green, rolling DB migration) | **`job_control`** command | Replaces Makefile path entirely |
| Bootstrap secrets before templates render | **`pre_deploy`** + `put_job_secret` | Secrets land in KV before rsync and `.tpl` render |
| Validate workspace or seed KV after catalog changes | **`post_build`** | Runs before deploy; build fails on error |
| Post-rollout smoke test or notification | **`post_deploy`** | Runs after successful start/restart |
| Liveness/readiness probe | **`health_check`** command and/or manifest `health_check` section (commands run after probes) | See [health-check.md](./health-check.md) |
| One-off ops (reindex, flush cache, admin task) | **`cli`** + `maand job_command` | Operator-triggered; commits KV on success |
| Run arbitrary shell on a worker | Python **`run_ssh`** / **`run_make_target`** / **`run_runner_target`** | Same SSH paths deploy uses; Bun has no SSH helpers |

**Do not** use job commands for long-running services — they are short-lived hooks. **Do not** expect scripts to run on workers by default: they execute in bash on the CLI host. Use **`run_ssh`** (Python) or stage files via deploy when workers must be touched.

**Disabled allocations** still receive job-command invocations (with `DISABLED=1`) so maintenance hooks can run; deploy does not start the job process on them. See [disabled.md](./disabled.md).

---

## CLI (ad-hoc)

```bash
maand job_command <job> <command_name> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--concurrency` | 1 | Max parallel allocations |
| `--verbose` | false | Stream command output to terminal and worker logs |

Example:

```bash
maand job_command api command_migrate --concurrency 2 --verbose
```

This always uses event **`cli`**. The command must be registered with **`executed_on`** including **`cli`** in the manifest.

Before running, maand verifies the command runtime on the **CLI host**: `python3` (or the job's `_modules/.venv` interpreter) for `.py` commands, `bun` for `.ts`/`.js` commands.

**When to use `cli`:** manual migrations, data fixes, operator runbooks, and debugging a command before wiring it into deploy events. KV changes commit to `maand.db` only after all allocations succeed.

---

## Command events (`executed_on`)

Build validates allowed values:

| Event | Triggered by | When to use |
|-------|----------------|-------------|
| **`post_build`** | End of **`maand build`** | Catalog validation, codegen, seeding `vars/job/*` before anyone deploys. **Build fails** if any hook fails. |
| **`pre_deploy`** | Start of job wave in **`maand deploy`** (before rsync) | Write secrets/vars consumed by `.tpl` on this deploy; pre-migration checks; fetch external config. |
| **`post_deploy`** | After successful rollout for that job | Smoke tests, cache warming, notifications, register service in external system. |
| **`job_control`** | Instead of Makefile start/restart/stop in **`maand deploy`** | Custom lifecycle: canary, leader election, coordinated multi-step start. |
| **`health_check`** | **`maand health_check`** and deploy after restart/job_control | Programmatic probes; must stay **KV read-only**. |
| **`cli`** | **`maand job_command`** only | Ad-hoc operator commands. |

A command may list **multiple** events (one row per event in the `job_commands` table). Register separate events when behavior differs (e.g. write in `pre_deploy`, read-only in `health_check`).

### Ordering and dependencies

**`demands`** in manifest:

```json
"demands": {
  "job": "database",
  "command": "command_schema",
  "config": {
    "min_version": "2.0.0",
    "max_version": "3.0.0"
  }
}
```

- **`demand_job` / `demand_command`**: Upstream job/command for deploy ordering; both required together when set.
- **`demands.config.min_version` / `max_version`**: Checked against upstream job **`version`** at build.
- Build computes **`deployment_seq`** and validates demand references. See [jobs-and-dependencies.md](./jobs-and-dependencies.md#job-version).
- **Self-reference** (`demand_job` = same job) is rejected at build.
- At runtime, scripts can call **`GET /demands`** to see which downstream commands depend on the current command and read their `demand_config`.

**Circular demands** fail **`maand build`**.

---

## File layout and naming

| Rule | Detail |
|------|--------|
| Command name | Must start with **`command_`** (e.g. `command_init`). |
| Script file | Exactly **one** of: `command_<name>.py`, `.ts`, or `.js` under `_modules/`. |
| Runtime | `.py` → `python3`; `.ts` / `.js` → `bun run`. |
| Embedded helpers | `maand.py` and `maand.ts` copied into staged `_modules/` at run time. |

Example:

```text
workspace/jobs/api/
  manifest.json
  _modules/
    command_init.py
    command_init.ts   # invalid: only one implementation allowed
```

---

## Manifest registration

```json
{
  "commands": {
    "command_init": {
      "executed_on": ["pre_deploy", "cli"],
      "demands": {
        "job": "",
        "command": "",
        "config": {}
      }
    }
  }
}
```

Inspect built jobs and dependency edges:

```bash
maand cat jobs
maand cat job_commands
```

See [jobs-and-dependencies.md](./jobs-and-dependencies.md).

---

## Execution model

```text
Open DB + kv.Initialize
StartRuntimeAPI (HTTP on localhost:8080 in maand process)
For each active allocation (worker IP):
  Stage tmp/workers/<ip>/jobs/<job>/ from job_files + certs from KV
  Run script on CLI host via bash (python3 or bun)
  Script reaches API at JOB_COMMAND_API_HOST=127.0.0.1
Commit (CLI path) or return error to caller (deploy/health_check)
```

Important details:

- **Where code runs:** bash on the **CLI host**, working directory = bucket root. Output is logged to `logs/<worker_ip>.log` (or `maand.log` for bucket-local).
- **Per-allocation context:** one process per allocation; env vars identify worker IP and allocation UUID even though the process is local.
- **Worker access:** use Python **`run_ssh`**, **`run_runner_target`**, or **`run_make_target`** to execute on `ALLOCATION_IP`. Bun scripts must shell out to `ssh` themselves or call a Python helper.
- **Staging:** `tmp/workers/<ip>/jobs/<job>/` mirrors deploy layout (files + certs) so scripts can read local copies if needed.

### Environment variables (job command scripts)

| Variable | Meaning |
|----------|---------|
| `ALLOCATION_ID` | Stable allocation UUID |
| `ALLOCATION_IP` | Worker host for this invocation |
| `JOB` | Job name |
| `EVENT` | Event name (`pre_deploy`, `cli`, …) |
| `COMMAND` | Command name |
| `DISABLED` | `0` or `1` — allocation marked disabled in catalog |
| `CURRENT_VERSION` | Running version on this allocation (`0.0.0` before first promote) |
| `NEW_VERSION` | Target version from the current build/deploy plan |
| `JOB_COMMAND_API_HOST` | Host to reach runtime API (`127.0.0.1`) |

Per-allocation KV also exposes `current_version` and `new_version` under `maand/job/<job>/worker/<ip>/`.

Deploy **`job_control`** also sets:

| Variable | Meaning |
|----------|---------|
| `NEW_ALLOCATIONS` | Comma-separated worker IPs getting a fresh promote |
| `UPDATED_ALLOCATIONS` | Comma-separated worker IPs getting a restart/update |

(`CURRENT_VERSION` and `NEW_VERSION` are set for all job command events, not only `job_control`.)

### Concurrency

- **`JobCommand(..., concurrency N)`** runs up to **N** allocations in parallel.
- Failures aggregate into a **run error** listing per-worker errors.
- Use **`acquire_semaphore`** when parallel allocations must serialize a critical section (migrations, external API with rate limits).

---

## Runtime HTTP API

Started by **`jobcommand.StartRuntimeAPI(tx)`** for the lifetime of build/deploy/health_check/job_command sessions.

| Property | Value |
|----------|--------|
| Listen address | **`localhost:8080`** (not exposed outside the host) |
| Request body | JSON, **`Content-Type: application/json`** |
| Allocation scope | Header **`X-ALLOCATION-ID`** (required on every route) |
| Event scope | Header **`EVENT`** (required; must match the running hook) |
| Command scope | Header **`COMMAND`** (required; used by `/demands` and semaphore scoping) |

Embedded **`maand.py`** / **`maand.ts`** set these headers automatically from env vars.

### Endpoint summary

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/kv` | Read a key from an allowed namespace |
| PUT | `/kv` | Write a non-secret key under **`vars/job/<current job>`** only |
| DELETE | `/kv` | Delete a key under **`vars/job/<current job>`** only |
| PUT | `/kv/secret` | Write encrypted secret under **`secrets/job/<current job>`** |
| DELETE | `/kv/secret` | Delete secret under **`secrets/job/<current job>`** |
| GET | `/kv/keys` | List keys under job-level namespaces |
| GET | `/demands` | List downstream commands that depend on this job+command |
| POST | `/semaphore/acquire` | Block until this allocation holds a slot |
| POST | `/semaphore/release` | Release a held slot |
| GET | `/semaphore/status?name=...` | Inspect holders and waiters |

### KV read vs write

| Namespace pattern | GET `/kv` | PUT/DELETE `/kv` | PUT/DELETE `/kv/secret` |
|-------------------|-----------|------------------|-------------------------|
| `maand`, `maand/worker`, `maand/worker/<ip>`, tags | ✓ | ✗ | ✗ |
| `vars/bucket`, `vars/bucket/job/<job>` | ✓ | ✗ | ✗ |
| `maand/job/<job>`, `maand/job/<job>/worker/<ip>` | ✓ | ✗ | ✗ |
| `vars/job/<job>` (current job) | ✓ | ✓ | ✗ |
| `secrets/job/<job>` (current job) | ✓ (decrypted) | ✗ | ✓ |
| Upstream demand jobs (`maand/job/*`, `vars/job/*`, `secrets/job/*`) | ✓ if declared in manifest **`demands`** | ✗ | ✗ |

Writes to **`maand/*`**, **`vars/bucket/*`**, and upstream jobs are rejected — use workspace files, `maand cat kv`, or the owning job's commands to change those namespaces.

**GET `/kv`** body:

```json
{ "namespace": "vars/job/api", "key": "db_url" }
```

Response (200):

```json
{ "namespace": "vars/job/api", "key": "db_url", "value": "postgres://..." }
```

**PUT `/kv`** body (value required):

```json
{ "namespace": "vars/job/api", "key": "db_url", "value": "postgres://..." }
```

**PUT `/kv/secret`** body:

```json
{ "namespace": "secrets/job/api", "key": "db_password", "value": "plain-text-secret" }
```

Values are encrypted with AES-256-GCM using `secrets/kv.key` before storage in `maand.db`.

**DELETE** `/kv` and `/kv/secret` use the same JSON body as GET (`namespace` + `key`; no `value`).

**GET `/kv/keys`** — optional body `{ "namespace": "vars/job/api" }`. Omit `namespace` to list both **`vars/job/<job>`** and **`secrets/job/<job>`** (secret listing returns key names only, never values).

### KV writes during `health_check`

**PUT**, **DELETE**, and **`/kv/secret`** writes are **rejected** when the **`EVENT`** header is **`health_check`**. Health check scripts must be read-only with respect to KV — use **`pre_deploy`** or **`post_deploy`** to create or update vars and secrets.

### `GET /demands`

Returns job commands whose manifest **`demands`** point at **this job** and **this command name** (reverse dependency lookup).

Response (200) — array of:

```json
{
  "job": "api",
  "command": "command_migrate",
  "demand_config": { "min_version": "2.0.0" }
}
```

**When to use:** a shared upstream command (e.g. `command_schema` on `database`) can inspect who depends on it and tailor behavior using `demand_config` (feature flags, schema versions, etc.).

### Semaphores

Coordinate cross-allocation locks inside one job command session. Scoped by **`job` + `EVENT` + name** — the same name under `pre_deploy` and `post_deploy` are independent semaphores.

| Field | Default | Limit |
|-------|---------|-------|
| `capacity` | 1 | 1–64 |
| `timeout_seconds` | 600 | max 3600 |

**POST `/semaphore/acquire`** body:

```json
{ "name": "migration", "capacity": 1, "timeout_seconds": 600 }
```

Response (200):

```json
{
  "name": "migration",
  "allocation_id": "<uuid>",
  "capacity": 1,
  "acquired": true
}
```

**POST `/semaphore/release`** body: `{ "name": "migration" }`

**GET `/semaphore/status?name=migration`** response:

```json
{
  "name": "migration",
  "capacity": 1,
  "holders": ["<allocation-uuid>"],
  "waiting": 0,
  "available": 0
}
```

**When to use semaphores:**

| Pattern | `capacity` | Example |
|---------|------------|---------|
| Single-writer migration | 1 | Only one allocation runs DDL at a time |
| Rolling batch | N | Allow N concurrent restarts against an external API |
| Leader bootstrap | 1 | First acquirer writes shared KV, others read |

Always **`release_semaphore`** in a `finally` / `try/finally` block so a failed script does not hold the lock for the rest of the deploy session.

Example (Python):

```python
from maand import acquire_semaphore, release_semaphore, allocation_ip

acquire_semaphore("migrate", capacity=1, timeout_seconds=900).raise_for_status()
try:
    run_migration_for(allocation_ip())
finally:
    release_semaphore("migrate")
```

Semaphores exist only in memory for the current maand process session — they do not survive CLI restart.

---

## Python and Bun helpers

Embedded **`maand.py`** / **`maand.ts`** wrap the HTTP API. Prefer these over raw HTTP.

### Context (env)

| Python | Bun | Returns |
|--------|-----|---------|
| `allocation_id()` | `allocationId()` | `ALLOCATION_ID` |
| `allocation_ip()` | `allocationIp()` | `ALLOCATION_IP` |
| `job_name()` | `jobName()` | `JOB` |
| `command_event()` | `commandEvent()` | `EVENT` |
| `command_name()` | `commandName()` | `COMMAND` |
| `is_allocation_disabled()` | `isAllocationDisabled()` | `DISABLED == "1"` |

Aliases: `get_allocation_id`, `get_job`, `kv_get`, etc. (both runtimes).

### KV

| Python | Bun | API |
|--------|-----|-----|
| `get_store_value(ns, key)` | `getStoreValue(ns, key)` | GET `/kv` → `Response` |
| `get_kv_value(ns, key)` | *(parse JSON yourself)* | GET `/kv` → plaintext `value` |
| `put_job_variable(key, val)` | `putJobVariable(key, val)` | PUT `/kv` |
| `put_job_secret(key, val)` | `putJobSecret(key, val)` | PUT `/kv/secret` |
| `delete_job_variable(key)` | `deleteJobVariable(key)` | DELETE `/kv` |
| `delete_job_secret(key)` | `deleteJobSecret(key)` | DELETE `/kv/secret` |
| `list_job_keys(ns=None)` | `listJobKeys(ns?)` | GET `/kv/keys` |

### Demands and semaphores

| Python | Bun | API |
|--------|-----|-----|
| `list_command_demands()` | `listCommandDemands()` | GET `/demands` |
| `acquire_semaphore(name, capacity=1, timeout_seconds=600)` | `acquireSemaphore(...)` | POST `/semaphore/acquire` |
| `release_semaphore(name)` | `releaseSemaphore(name)` | POST `/semaphore/release` |
| `semaphore_status(name)` | `semaphoreStatus(name)` | GET `/semaphore/status` |

### Worker SSH (Python only)

| Function | Purpose |
|----------|---------|
| `load_ssh()` | Parse `maand.conf` → `(user, key_path, use_sudo)` |
| `run_ssh(worker_ip, remote_cmd, ...)` | Arbitrary remote command over SSH |
| `run_runner_target(target, ...)` | `runner.py <target> --jobs <job>` on worker (same as deploy) |
| `run_make_target(target, ...)` | `make -C /opt/worker/<bucket>/jobs/<job> <target>` |

Bun scripts that need SSH should invoke **`ssh`** directly or call a thin Python wrapper script.

### Python virtualenv (recommended)

Create a venv **per job** under `_modules` (not copied into `tmp/` during runs; maand calls the workspace interpreter):

```bash
cd workspace/jobs/<job>/_modules
python3 -m venv .venv
source .venv/bin/activate   # optional for manual work
pip install -r requirements.txt
pip install requests        # required if scripts use maand.py
```

Maand uses, in order:

1. `workspace/jobs/<job>/_modules/.venv/bin/python3`
2. `workspace/jobs/<job>/_modules/venv/bin/python3`
3. `python3` on your PATH

`.venv`, `venv`, `node_modules`, and `__pycache__` are skipped during **`maand build`** file indexing.

### Bun

Install [Bun](https://bun.sh) on the CLI host. Per job:

```bash
cd workspace/jobs/<job>/_modules
bun install
```

---

## KV persistence by context

| Context | When KV writes persist to `maand.db` |
|---------|--------------------------------------|
| **`maand build`** | End of main build transaction (`PersistToTransaction`). |
| **`post_build`** hooks | Separate session transaction at end of hook pass (failures fail build). |
| **`maand job_command`** | Successful CLI commit. |
| **`maand deploy`** | **`kv.PersistSession()`** after each job's `pre_deploy` and after each **`deployJob`**. |
| **`maand health_check`** | KV writes rejected (read-only). |

Use **`pre_deploy`** to write secrets via **`put_job_secret`** / **`putJobSecret`** or plain vars via **`put_job_variable`**, consumed by **`.tpl`** files on the same deploy (`getSecret` / `get`).

See [kv.md](./kv.md#persistence-timing) for the full persistence model.

---

## Event-specific behavior (detail)

### `post_build`

- Runs on the **CLI host** after the main catalog commit, in **`deployment_seq`** order across all jobs.
- **When to use:** validate generated artifacts, run codegen, populate **`vars/job/*`** before any deploy, cross-job consistency checks.
- **Any hook failure fails `maand build`.**
- KV writes persist in the post-build hook transaction.

### `pre_deploy`

- Runs on the **CLI host** before rsync for jobs that need rollout.
- **When to use:** fetch secrets from Vault, write **`secrets/job/*`**, set vars for templates, verify external prerequisites.
- Failure: job skipped for this deploy wave; KV written before failure is still checkpointed if the persist step succeeds.

### `post_deploy`

- After start/restart or **`job_control`** path succeeds through health check (if any).
- **When to use:** smoke tests, warm caches, register with service discovery, send notifications.
- Failure: job error; partial deploy may leave other jobs promoted.

### `job_control`

- Replaces Makefile **`runner.py`** path entirely for that job's deploy wave.
- **When to use:** canary deploys, custom stop/start ordering, application-specific rolling logic.
- You must implement rollout using `NEW_ALLOCATIONS` / `UPDATED_ALLOCATIONS` and typically **`run_runner_target`** or **`run_ssh`**.
- Still followed by **health_check** (if configured) and **post_deploy**.

### `health_check`

- See [`health-check.md`](./health-check.md).
- **When to use:** HTTP/TCP/custom probes that need code rather than a static manifest probe.
- **Mutually exclusive** with a manifest **`health_check`** section on the same job (build fails if both).
- **KV read-only** — any write returns HTTP 400.
- CLI **`--update-hash`** on **`maand health_check`** marks allocations whose **command** failed for redeploy; follow with **`maand deploy`** (or **`maand deploy --force`**).

### `cli`

- Only **`maand job_command`**.
- **When to use:** operator-run maintenance, testing commands before adding deploy events, one-off data migration.
- Commits transaction on success.

---

## Default deploy without `job_control`

If no `job_control` commands, deploy does **not** run arbitrary `executed_on` events except:

- `pre_deploy` (before stage)
- `post_deploy` (after rollout)
- implicit **health_check** if defined

Lifecycle uses **Makefile** via **`runner.py`** on workers, not your command scripts.

---

## Example: secret bootstrap + template

**`command_bootstrap.py`** (`pre_deploy`):

```python
from maand import put_job_secret, put_job_variable

put_job_secret("db_password", fetch_from_vault()).raise_for_status()
put_job_variable("db_host", "db.internal").raise_for_status()
```

**`config.toml.tpl`** (deploy renders on worker):

```toml
db_host = "{{ get "vars/job/api" "db_host" }}"
db_password = "{{ getSecret "secrets/job/api" "db_password" }}"
```

---

## Writing a new command (checklist)

1. Add **`command_myaction.py`** (or `.ts`) under `_modules/`.
2. Register in **`manifest.json`** with correct **`executed_on`** and **`demands`** if needed.
3. Run **`maand build`**.
4. Test with **`maand job_command <job> command_myaction --verbose`** if `cli` is listed.
5. Wire into deploy by adding `pre_deploy` / `post_deploy` / `job_control` as needed.
6. Use KV namespaces allowed for templates if scripts set vars for `.tpl` files.

---

## Errors

### Maand CLI / orchestration

| Error | Meaning |
|-------|---------|
| `NotFoundError` | Command not allowed for this event on this job. |
| `RunError` | One or more allocations failed (see wrapped per-worker errors). |
| `WorkerFailure` | Script exited non-zero or SSH failure on a worker. |
| File not found | Missing or duplicate `.py`/`.ts`/`.js` implementation. |

### Runtime HTTP API

| HTTP | Message | Typical cause |
|------|---------|---------------|
| 400 | `X-ALLOCATION-ID header is missing` | Raw HTTP call without header |
| 404 | `Invalid allocation ID` | Stale or wrong allocation UUID |
| 400 | `Both namespace and key are required` | Incomplete JSON body |
| 400 | `Invalid or unauthorized namespace` | Write to read-only namespace, wrong job, or upstream not in demands |
| 404 | `KV get operation failed` | Key does not exist |
| 400 | `KV writes are not allowed during health_check` | PUT/DELETE during health_check event |
| 408 | `Timed out waiting for semaphore` | `timeout_seconds` elapsed |
| 409 | `Semaphore acquire or release failed` | Release without hold, or internal conflict |
| 415 | `Content-Type must be application/json` | Missing or wrong content type |
| 400 | `Invalid JSON format` | Malformed request body |

Check **`logs/<worker_ip>.log`** and CLI output when **`--verbose`** is set.

---

## Related commands

| Command | Doc |
|---------|-----|
| `maand build` | [`build.md`](./build.md) |
| `maand deploy` | [`deploy.md`](./deploy.md) |
| `maand health_check` | [`health-check.md`](./health-check.md) |
| `maand job start\|stop\|restart` | Makefile runner via **jobcontrol** (not job_command events) |

---

## Quick reference: who runs what

```text
                    post_build   pre_deploy   deploy roll   post_deploy   health_check   cli
maand build              ✓            —            —            —              —         —
maand deploy             —            ✓            ✓*           ✓              ✓*        —
maand health_check       —            —            —            —              ✓         —
maand job_command        —            —            —            —              —         ✓

* deploy: health_check after restart/job_control; roll = job_control OR Makefile path
```
