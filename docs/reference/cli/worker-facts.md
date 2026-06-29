# `maand worker_facts`

**worker_facts** SSHes to workers listed in **`workspace/workers.json`**, reads host memory and CPU capacity, and writes the values back to **`workers.json`**. Only **`memory`** and **`cpu`** are updated; other fields (`host`, `labels`, `tags`, `hostname`, `position`) are preserved.

Use this when onboarding new hosts or when hardware changes and you need accurate capacity for build-time resource validation.

## CLI

```bash
maand worker_facts [flags]
```

| Flag | Description |
|------|-------------|
| `--workers` | `-w` | Comma-separated worker IPs (default: all entries in `workers.json`). |
| `--labels` | `-l` | Comma-separated worker labels (any match). |
| `--concurrency` | `-c` | Workers to probe in parallel (default: 1). |
| `--dry-run` | | Print planned changes; do not write `workers.json`. |
| `--build` | | Run **`maand build`** after a successful update. |

Examples:

```bash
maand worker_facts --dry-run
maand worker_facts
maand worker_facts -w 10.0.0.1,10.0.0.2 -c 2
maand worker_facts --build
```

## What gets probed

| Field | Source on worker |
|-------|------------------|
| **memory** | `MemTotal` from `/proc/meminfo` → `"15839 mb"` |
| **cpu** | Logical cores (`nproc`) × per-core MHz from `/proc/cpuinfo` or `lscpu` → `"10804 mhz"` |

CPU is stored as **total capacity in MHz** (cores × MHz per core), matching how build sums job reservations against worker capacity.

## Output format

Updated entries omit empty optional fields. A minimal worker after probe:

```json
{
  "host": "10.0.0.1",
  "memory": "15839 mb",
  "cpu": "10804 mhz"
}
```

Workers with existing **`labels`**, **`tags`**, or **`hostname`** keep them; unset fields are not written as `null`.

## Prerequisites

- Initialized bucket with worker **`host`** entries in **`workspace/workers.json`** (memory/cpu may be empty before the first run).
- SSH key at `secrets/<ssh_key>` (from `maand.conf`) authorized on workers.
- Host tools: `bash`, `ssh` (checked before SSH).
- Target workers: Linux with `/proc/meminfo`, `bash`, and `timeout` (`sudo` when `use_sudo = true`).

Unlike **`maand run_command`**, **`worker_facts`** does not require a prior **`maand build`** — it reads and writes the workspace file directly. Run **`maand build`** (or use **`--build`**) afterward so **`maand.db`** and KV reflect the new capacity.

## Behavior

1. Load workers from **`workspace/workers.json`** (apply **`--workers`** / **`--labels`** filters).
2. SSH to each target and run the probe script.
3. Compare probed values to the file; print a line per changed worker.
4. On success, rewrite **`workers.json`** (skipped with **`--dry-run`**).
5. Optionally run **`maand build`**.

If any worker probe fails, **nothing is written** (all targets must succeed).

When values are already correct:

```text
workers.json already up to date
```

## Typical workflow

```bash
# Add hosts to workspace/workers.json (host + labels only is fine)
maand worker_facts --dry-run
maand worker_facts --build
```

Or without **`--build`**:

```bash
maand worker_facts
maand build
```

## Notes

- **`--workers`** and **`--labels`** are mutually exclusive (same as **`maand run_command`**).
- This command does not contact workers for any purpose other than the capacity probe.
- For arbitrary shell on workers, use **`maand run_command`** — see [run-command.md](run-command.md).
