# Job commands

**Job commands** are scripts (Python or Bun) under `workspace/jobs/<job>/_modules/` that maand runs on the **CLI host** for each **allocation** (job on a worker). They access configuration through a **runtime HTTP API** backed by the in-memory **KV store** loaded from `maand.db`.

Commands are declared in **`manifest.json`** with an **`executed_on`** list that controls when maand may run them.

---

## CLI (ad-hoc)

```bash
maand job_command <job> <command_name> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--concurrency` | 1 | Max parallel workers |
| `--verbose` | false | Stream command output |

Example:

```bash
maand job_command api command_migrate --concurrency 2 --verbose
```

This always uses event **`cli`**. The command must be registered with **`executed_on`** including **`cli`** in the manifest.

Before running, maand verifies the command runtime on the **CLI host**: `python3` (or the job's `_modules/.venv` interpreter) for `.py` commands, `bun` for `.ts`/`.js` commands.

---

## Command events (`executed_on`)

Build validates allowed values:

| Event | Triggered by |
|-------|----------------|
| **`post_build`** | End of **`maand build`** (separate tx; **build fails** if any hook fails). |
| **`pre_deploy`** | Start of job wave in **`maand deploy`** (before staging). |
| **`post_deploy`** | After successful rollout for that job in **`maand deploy`**. |
| **`job_control`** | Instead of default Makefile start/restart/stop in **`maand deploy`**. |
| **`health_check`** | **`maand health_check`** and deploy after restart/job_control. |
| **`cli`** | **`maand job_command`** only. |

A command may list **multiple** events (one row per event in `job_commands` table).

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
- At runtime, scripts can call **`GET /demands`** to see dependent commands and `demand_config`.

**Circular demands** fail **`maand build`**.

---

## File layout and naming

| Rule | Detail |
|------|--------|
| Command name | Must start with **`command_`** (e.g. `command_init`). |
| Script file | Exactly **one** of: `command_<name>.py`, `.ts`, or `.js` under `_modules/`. |
| Runtime | `.py` → `python3`; `.ts` / `.js` → `bun run`. |
| Embedded helpers | `maand.py` and `maand.ts` copied into `_modules/` at run time. |

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
  Run script on CLI host (python3 or bun)
Commit (CLI path) or return error to caller (deploy/health_check)
```

### Environment variables (job command scripts)

| Variable | Meaning |
|----------|---------|
| `ALLOCATION_ID` | Stable allocation UUID |
| `ALLOCATION_IP` | Worker host |
| `JOB` | Job name |
| `EVENT` | Event name (`pre_deploy`, `cli`, …) |
| `COMMAND` | Command name |
| `DISABLED` | `0` or `1` |
| `CURRENT_VERSION` | Running version on this allocation (`0.0.0` before first promote) |
| `NEW_VERSION` | Target version from the current build/deploy plan |
| `JOB_COMMAND_API_HOST` | Host to reach runtime API (`127.0.0.1`) |

Per-allocation KV also exposes `current_version` and `new_version` under `maand/job/<job>/worker/<ip>/`.

Deploy **`job_control`** also sets:

| Variable | Meaning |
|----------|---------|
| `NEW_ALLOCATIONS` | Comma-separated worker IPs |
| `UPDATED_ALLOCATIONS` | Comma-separated worker IPs |

(`CURRENT_VERSION` and `NEW_VERSION` are set for all job command events, not only `job_control`.)

### Concurrency

- **`JobCommand(..., concurrency N)`** runs up to **N** allocations in parallel.
- Failures aggregate into a **run error** listing per-worker errors.

---

## Runtime HTTP API

Started by **`jobcommand.StartRuntimeAPI(tx)`** for the lifetime of build/deploy/health_check/job_command.

Listen: **`localhost:8080`** (not exposed outside the host).

Scripts use header **`X-ALLOCATION-ID`** so the server scopes KV and demands to the correct allocation.

### `GET` / `PUT` `/kv`

Body JSON:

```json
{ "namespace": "vars/job/api", "key": "db_url", "value": "postgres://..." }
```

- **GET**: Read key (must be in **allowed namespaces** for that job/worker). Values under `secrets/job/<job>` are returned decrypted.
- **PUT**: Write a non-secret key under `vars/job/<job>` (in-memory store; persisted when deploy checkpoints or build/CLI commits).

### `PUT` `/kv/secret`

Write an encrypted secret for the current job:

```json
{ "namespace": "secrets/job/api", "key": "db_password", "value": "plain-text-secret" }
```

Values are encrypted with AES-256-GCM using `secrets/kv.key` before they are stored in `maand.db`. Python: **`put_job_secret(key, value)`**. Bun: **`putJobSecret(key, value)`**.

### `DELETE` `/kv` and `/kv/secret`

Delete a key (same JSON body as GET: `namespace` + `key`). Vars use `/kv`; secrets use `/kv/secret`. Python: **`delete_job_variable(key)`**, **`delete_job_secret(key)`**.

### `GET` `/kv/keys`

List keys at job level. Optional body `{ "namespace": "vars/job/api" }`; omit `namespace` to list both **`vars/job/<job>`** and **`secrets/job/<job>`** (key names only for secrets, not values). Python: **`list_job_keys()`**. Bun: **`listJobKeys()`**.

### KV writes during `health_check`

**PUT**, **DELETE**, and **`/kv/secret`** writes are **rejected** when the **`EVENT`** header is **`health_check`**. Health check scripts must be read-only with respect to KV — use **`pre_deploy`** or **`post_deploy`** to create or update vars and secrets.

Allowed namespaces (typical):

- `maand`, `maand/worker`, `maand/worker/<ip>`, `maand/worker/<ip>/tags`
- `maand/job/<job>`, `vars/job/<job>`, `vars/bucket/job/<job>`, `secrets/job/<job>`
- `maand/job/<job>/worker/<ip>`

### `GET` `/demands`

Returns dependent commands for the calling allocation’s job/event/command (from `job_commands` + manifest demands).

### Semaphores `/semaphore/acquire`, `/release`, `/status`

Coordinate cross-allocation locks (e.g. single-writer migration):

- **POST** `/semaphore/acquire` — body: `name`, optional `capacity`, `timeout_seconds`.
- **POST** `/semaphore/release` — body: `name`.
- **GET** `/semaphore/status` — holders and waiters.

Scoped per job/event via request headers **`EVENT`**, **`COMMAND`**, **`X-ALLOCATION-ID`**.

---

## Python and Bun helpers

Embedded **`maand.py`** / **`maand.ts`** wrap the HTTP API (KV, demands, semaphores). Prefer these over raw HTTP in command scripts.

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

Install [Bun](https://bun.sh) on the host. Per job:

```bash
cd workspace/jobs/<job>/_modules
bun install
```

---


## KV persistence by context

| Context | When KV writes persist to `maand.db` |
|---------|--------------------------------------|
| **`maand build`** | End of main build transaction (`PersistToTransaction`). |
| **`maand job_command`** | Successful CLI commit. |
| **`maand deploy`** | **`kv.PersistSession()`** after each job’s `pre_deploy` and after each **`deployJob`** (separate connection; survives deploy tx rollback). |
| **`maand health_check`** | With deploy/CLI transaction commit. |
| **`post_build`** | Does not commit hook transaction KV to DB unless you persist via API and a later build/deploy saves it. |

Use **`pre_deploy`** to write secrets via **`put_job_secret`** / **`putJobSecret`** or plain vars via **`put_job_variable`**, consumed by **`.tpl`** files on the same deploy (`getSecret` / `get`).

---

## Event-specific behavior (detail)

### `post_build`

- Runs after catalog commit.
- Good for: validation, generating artifacts, dry-run tests.
- **Failure does not fail build.**

### `pre_deploy`

- Runs before rsync for jobs that need rollout.
- Failure: job skipped for this deploy wave; KV still checkpointed if written before failure (checkpoint runs after hook returns in execute loop — check code: persist runs after pre_deploy even on failure?)

From execute.go:
```go
preErr := executePreJobCommandsForJob(...)
if persistErr := persistJobCommandKV(job); ...
if preErr != nil { continue }
```
So KV from a failing pre_deploy is still persisted if PersistSession succeeds.

### `post_deploy`

- After start/restart or job_control path succeeds through health check (if any).
- Failure: job error; partial deploy may leave other jobs promoted.

### `job_control`

- Replaces Makefile runner path entirely.
- You must implement rollout using `NEW_ALLOCATIONS` / `UPDATED_ALLOCATIONS` env vars.
- Still followed by **health_check** (wait) and **post_deploy**.

### `health_check`

- See [`health-check.md`](./health-check.md).

### `cli`

- Only **`maand job_command`**.
- Commits transaction on success.

---

## Default deploy without `job_control`

If no `job_control` commands, deploy does **not** run arbitrary `executed_on` events except:

- `pre_deploy` (before stage)
- `post_deploy` (after rollout)
- implicit **health_check** if defined

Lifecycle uses **Makefile** via **`runner.py`** on workers, not your command scripts.

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

| Error | Meaning |
|-------|---------|
| `NotFoundError` | Command not allowed for this event on this job. |
| `RunError` | One or more workers failed (see wrapped errors). |
| `WorkerFailure` | SSH/script failure on a worker. |
| File not found | Missing or duplicate `.py`/`.ts`/`.js` implementation. |
| Runtime API 4xx | Bad namespace, missing allocation header, semaphore timeout. |

---

## Related commands

| Command | Doc |
|---------|-----|
| `maand build` | [`build.md`](./build.md) |
| `maand deploy` | [`deploy.md`](./deploy.md) |
| `maand health_check` | [`health-check.md`](./health-check.md) |
| `maand job start|stop|restart` | Makefile runner via **jobcontrol** (not job_command events) |

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
