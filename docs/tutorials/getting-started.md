# Tutorial: Getting started

This walkthrough creates a maand bucket, registers two workers, adds a simple job, builds the catalog, and deploys it. You need:

- **CLI host:** `maand` binary built with `CGO_ENABLED=1`, plus `bash`, `ssh`, `rsync`, `python3`
- **Workers:** Linux hosts with SSH access, `python3`, `make`, `rsync`, `bash`, `timeout` on `PATH`
- **SSH key:** private key in `secrets/` authorized for `ssh_user` on each worker

Concepts: [concepts.md](../concepts.md) · Command list: [commands.md](../commands.md)

---

## Step 1 — Create the bucket

Choose a directory for your project and initialize:

```bash
mkdir my-cluster && cd my-cluster
maand init
```

You should see `maand bucket initialized`. Verify layout:

```bash
ls -la
# maand.conf  data/  workspace/  secrets/  tmp/  logs/
cat workspace/workers.json
# []
```

Copy your worker SSH private key:

```bash
cp ~/.ssh/id_ed25519 secrets/worker.key
chmod 600 secrets/worker.key
```

Edit **`maand.conf`** if needed (defaults assume user `agent`, key `worker.key`, `use_sudo = true`):

```toml
ssh_user = "agent"
ssh_key = "worker.key"
use_sudo = true
```

Ensure the matching public key is in `~agent/.ssh/authorized_keys` on every worker.

---

## Step 2 — Register workers

Edit **`workspace/workers.json`**:

```json
[
  {
    "host": "10.0.0.1",
    "labels": ["worker"],
    "memory": "4096 mb",
    "cpu": "2000 mhz"
  },
  {
    "host": "10.0.0.2",
    "labels": ["worker"],
    "memory": "4096 mb",
    "cpu": "2000 mhz"
  }
]
```

Every worker automatically gets the **`worker`** label. Jobs use **selectors** to decide which workers receive an allocation.

Test SSH from the CLI host:

```bash
ssh -i secrets/worker.key agent@10.0.0.1 echo ok
ssh -i secrets/worker.key agent@10.0.0.2 echo ok
```

---

## Step 3 — Create a job

Scaffold a job:

```bash
maand job create hello --selectors worker
```

This creates `workspace/jobs/hello/` with `manifest.json` and a minimal **Makefile**. Replace the Makefile with something that tracks lifecycle (deploy calls `make start` / `make restart`):

```bash
cat > workspace/jobs/hello/Makefile <<'EOF'
.PHONY: start stop restart status

dir:
	mkdir -p ./data ./logs ./bin

start: dir
	@echo "hello started" > ./data/status
	@date >> ./logs/start.log

stop:
	@echo stopped > ./data/status

restart: stop start

status:
	@cat ./data/status 2>/dev/null || echo not running
EOF
```

Ensure **`manifest.json`** includes selectors (the `create` command writes `"selectors": ["worker"]`):

```json
{
  "version": "1.0.0",
  "selectors": ["worker"],
  "update_parallel_count": 1,
  "resources": {
    "memory": { "min": "64 mb", "max": "256 mb" },
    "cpu": { "min": "100 mhz", "max": "500 mhz" }
  }
}
```

Do **not** commit `data/`, `logs/`, or `bin/` under the job folder in workspace — those are created on workers at runtime.

---

## Step 4 — Build the catalog

```bash
maand build
```

Build reads workers and jobs, creates **allocations** (hello × each worker), fills KV, and validates resources. Inspect:

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
```

You should see two allocations: `hello` on `10.0.0.1` and `hello` on `10.0.0.2`, both active (`disabled=0`, `removed=0`).

If build fails, common causes:

- Missing Makefile under the job
- Port collision between jobs
- Job memory/CPU exceeds worker capacity
- Invalid `workers.json` (duplicate hosts, bad units)

See [build.md](../build.md#common-errors).

---

## Step 5 — Deploy to workers

```bash
maand deploy
```

Deploy will:

1. Check host tools (`ssh`, `rsync`, `python3`, …) and worker prerequisites
2. Stage job files under `tmp/workers/<ip>/`
3. Rsync to `/opt/worker/<bucket_id>/` on each worker
4. Run `make start` (first deploy) or `make restart` (content changed)

Optional: combine build + deploy:

```bash
maand deploy --build
```

Dry-run before a production push:

```bash
maand deploy --dry-run
```

Deploy only one job:

```bash
maand deploy --jobs hello
```

---

## Step 6 — Verify on workers

Check remote state with **`maand run_command`**:

```bash
maand run_command "cat /opt/worker/*/jobs/hello/data/status"
maand run_command "hostname && cat /opt/worker/*/jobs/hello/data/status"
```

Or SSH manually:

```bash
ssh agent@10.0.0.1 "cat /opt/worker/*/jobs/hello/data/status"
```

Use **`maand job status`** (requires deploy to have synced `update_seq`):

```bash
maand job status hello
maand job status hello --allocations 10.0.0.1
```

---

## Step 7 — Change and roll out

Edit the job (for example bump `version` in `manifest.json` or change the Makefile), then:

```bash
maand build
maand deploy
```

Deploy compares **content hashes** per allocation and restarts only jobs that changed. With `update_parallel_count: 2`, restarts happen in rolling batches of two workers.

Each **`make restart`** receives **`CURRENT_VERSION`** (what was last promoted on that worker) and **`NEW_VERSION`** (the target from your manifest after build). Use them in the Makefile for migrations or upgrade scripts — see [deploy.md](../deploy.md#allocation-version-tracking).

Bump **`version`** in `manifest.json` when you want templates, KV, and deploy to reflect a new release. Bumping version alone still triggers rollout when `manifest.json` is part of the synced tree (content hash changes). If this job depends on others (or is depended on), use semver strings and optional **`min_version`** / **`max_version`** in **`demands.config`** — see [jobs-and-dependencies.md](../jobs-and-dependencies.md#job-version).

---

## What you have now

| Piece | State |
|-------|--------|
| **Bucket** | Local project with `maand.db` |
| **Workers** | Two SSH targets in the catalog |
| **Job** | `hello` with Makefile lifecycle |
| **Allocations** | `hello` on each worker |
| **Worker paths** | `/opt/worker/<bucket_id>/jobs/hello/` |

---

## Next steps

- [Day-2 operations](./day-2-operations.md) — disable allocations, health checks, GC, manual job control
- [Job commands tutorial](./job-commands.md) — Python/Bun hooks and KV
- [job-command.md](../job-command.md) — runtime API reference
- [deploy.md](../deploy.md) — rollout, hooks, dry-run, allocation version tracking
