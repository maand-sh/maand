# Job command runtime API

Python/Bun scripts reach maand through an in-process HTTP API on the CLI host. For when to use hooks and event names, see [cli/job-command.md](./cli/job-command.md).

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

- **Where code runs:** bash on the **CLI host**, working directory = bucket root. Output is logged to structured lines in `logs/<worker_ip>.log` (or `logs/maand.log` for bucket-local). Each line includes timestamp, run id, command phase, and payload. Per-run copies live under `logs/runs/<run_id>/`.
- **Per-allocation context:** one process per allocation; env vars identify worker IP and allocation UUID even though the process is local.
- **Worker access:** use Python **`run_ssh`**, **`run_runner_target`**, or **`run_make_target`** to execute on `ALLOCATION_IP`. Bun scripts must shell out to `ssh` themselves or call a Python helper.
- **Staging:** `tmp/workers/<ip>/jobs/<job>/` mirrors deploy layout (files + certs) so scripts can read local copies if needed.

### Environment variables (job command scripts)

| Variable | Meaning |
|----------|---------|
| `ALLOCATION_ID` | Stable allocation UUID |
| `ALLOCATION_IP` | Worker host for this invocation |
| `ALLOCATION_INDEX` | Zero-based index among non-removed peers (same as `<job>_allocation_index` in per-allocation KV) |
| `JOB` | Job name |
| `EVENT` | Event name (`pre_deploy`, `cli`, …) |
| `COMMAND` | Command name |
| `DISABLED` | `0` or `1` — allocation marked disabled in catalog |
| `CURRENT_VERSION` | Running version on this allocation (`0.0.0` before first promote) |
| `NEW_VERSION` | Target version from the current build/deploy plan |
| `JOB_COMMAND_API_HOST` | Host to reach runtime API (`127.0.0.1`) |

Per-allocation KV exposes **`version`** (build target) under `maand/job/<job>/worker/<ip>/`. Running vs target for rollout logic lives in the catalog (`hash.current_version`, `allocations.new_version`) and template fields **`.CurrentVersion`** / **`.NewVersion`**.

Deploy **`job_control`** also sets:

| Variable | Meaning |
|----------|---------|
| `NEW_ALLOCATIONS` | Comma-separated worker IPs getting a fresh promote |
| `UPDATED_ALLOCATIONS` | Comma-separated worker IPs getting a restart/update |

(`CURRENT_VERSION` and `NEW_VERSION` are set for all job command events, not only `job_control`.)

**`after_allocation_started`** / **`after_allocation_stopped`** also receive batch rollout env (in addition to per-allocation vars):

| Variable | Meaning |
|----------|---------|
| `BATCH_ALLOCATIONS` | Comma-separated worker IPs in this batch |
| `BATCH_INDEX` | Zero-based batch index |
| `BATCH_COUNT` | Total batches in the current phase |
| `DEPLOY_PHASE` | `new`, `update`, or `stop` |
| `DEPLOY_ORDER` | Full comma-separated order list |
| `DEPLOY_ORDER_SOURCE` | `kv` or `default` |
| `JOB` | Job name (same as env above; set again for batch hooks) |

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
| `maand/bucket`, `maand/worker`, `maand/worker/<ip>`, tags | ✓ | ✗ | ✗ |
| `vars/bucket`, `vars/bucket/job/<job>` | ✓ | ✗ | ✗ |
| `maand/job/<job>/worker/<ip>` | ✓ | ✗ | ✗ |
| `maand/job/<job>` key **`deploy_order`** only | ✓ | ✓ on **`pre_deploy`** or **`cli`** | ✗ |
| `vars/job/<job>` (current job) | ✓ | ✓ | ✗ |
| `secrets/job/<job>` (current job) | ✓ (decrypted) | ✗ | ✓ |
| Upstream demand jobs (`maand/job/*`, `vars/job/*`, `secrets/job/*`) | ✓ if declared in manifest **`demands`** | ✗ | ✗ |

Writes to other **`maand/*`** keys, **`vars/bucket/*`**, and upstream jobs are rejected. Use **`put_deploy_order`** / **`putDeployOrder`** (or PUT `/kv` on `maand/job/<job>` + key `deploy_order`) to override rollout order for one deploy; build resets it on the next **`maand build`**.

**PUT `deploy_order`** body:

```json
{
  "namespace": "maand/job/api",
  "key": "deploy_order",
  "value": "10.0.0.2,10.0.0.1"
}
```

```python
from maand import put_deploy_order

put_deploy_order(["10.0.0.2", "10.0.0.1"]).raise_for_status()
```

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
| `allocation_index()` | `allocationIndex()` | `ALLOCATION_INDEX` |
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
| `put_deploy_order(order)` | `putDeployOrder(order)` | PUT `/kv` → `maand/job/<job>/deploy_order` |
| `get_deploy_order()` | `getDeployOrder()` | GET `/kv` → `deploy_order` |
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

Use **`pre_deploy`** to write secrets consumed by **`.tpl`** on the same deploy. Full persistence and purge rules: [kv/persistence.md](./kv/persistence.md). Namespace keys: [kv/namespaces.md](./kv/namespaces.md).

---

## HTTP API errors

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

## Related

- [cli/job-command.md](./cli/job-command.md) — events, patterns, checklist
- [kv/namespaces.md](./kv/namespaces.md) · [kv/persistence.md](./kv/persistence.md)
- [cli/build.md](./cli/build.md) · [cli/deploy.md](./cli/deploy.md)
