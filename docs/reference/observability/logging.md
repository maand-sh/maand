# Logging and observability

Maand writes structured logs for deploy, rsync, SSH, job commands, GC, health checks, and related operations.

Related: [configuration.md](../configuration.md#maandconf-bucket-root) (`log_format`) · [debugging-deploy.md](../../guides/debugging-deploy.md)

---

## Log files

| Path | Contents |
|------|----------|
| `logs/<worker_ip>.log` | All lines for commands targeting that worker (append-only) |
| `logs/maand.log` | Bucket-local commands and job-level events (`worker=-`) |
| `logs/runs/<run_id>/` | Copy of one CLI invocation (same line format per worker) |

Every `maand` command gets a **run id** (UUID). Begin/end lines include `run=<uuid>` so you can correlate workers touched in one invocation.

---

## On-disk format

Set **`log_format`** in **`maand.conf`**:

| Value | Encoding |
|-------|----------|
| `kv` (default) | Space-separated `key=value` lines |
| `json` or `jsonl` | One JSON object per line |

### Command boundaries

```text
ts=2026-06-02T22:07:01.123Z level=INFO event=command_begin run=<uuid> maand=deploy seq=17 worker=10.48.200.3 job=postgres phase=reconcile action=stop cmd="python3 ... runner.py stop postgres"
ts=2026-06-02T22:07:01.200Z level=INFO run=<uuid> maand=deploy seq=17 worker=10.48.200.3 job=postgres phase=reconcile action=stop stream=stdout msg="Container postgres  Stopping"
ts=2026-06-02T22:07:02.456Z level=INFO event=command_end run=<uuid> maand=deploy worker=10.48.200.3 job=postgres phase=reconcile action=stop exit=0 duration_ms=1333
```

Subprocess stdout/stderr are **structured stream lines** with `stream=stdout` or `stream=stderr` and `msg=...`. Legacy log files may still contain payload-only `| msg` / `! msg` lines; parsers accept both.

### Events

Common **`event`** values: `command_begin`, `command_end`, `deploy_skip`, `reconcile_skip_stop`.

Common **`phase`** values: `reconcile`, `rsync`, `rollout`, `job_command`, `run_command`, `gc`, `post_build`, `validate`, `job_control`.

Job-level messages use **`worker=-`**. Per-worker commands include **`worker=<ip>`** and usually **`job=<name>`**.

---

## Terminal output (live CLI)

While a command runs, maand prints a **human-readable** stream view to stderr (via the Go log package):

```text
postgres@10.48.200.3 | Container postgres  Stopping
cassandra@10.48.200.1 | seed: 10.48.200.1
jobcommand: zookeeper
zookeeper@10.48.200.1 | ensemble: 3 nodes
```

- Stream lines: **`job@worker | message`** (stderr uses **`!`**)
- Job-only context: **`job | message`**
- Multi-job **`maand jobcommand`**: **`jobcommand: <job>`** header before each job’s block

On-disk logs keep the full structured fields; the terminal view is for readability.

Deploy skip lines remain plain text plus a structured event, for example:

```text
deploy: skip job "cassandra" (already promoted on all allocations)
ts=... event=deploy_skip ... worker=- job=cassandra reason=already_promoted
```

---

## `maand logs show`

```bash
maand logs show [flags]
```

| Flag | Description |
|------|-------------|
| `--format` | **`raw`** (default): lines as stored; **`human`**: grouped command blocks |
| `--worker` | Filter by worker IP (omit to search all `logs/*.log`) |
| `--run` | Filter by run id; with `--run-dir`, read from `logs/runs/<run>/` |
| `--job` | Filter by job name |
| `--phase` | Filter by phase |
| `--event` | Filter by event name |
| `--tail N` | Last N matching lines (`raw`) or command blocks (`human`) |
| `--run-dir` | Read per-run files under `logs/runs/<run>/` instead of aggregate logs |

### Human format

```bash
maand logs show --worker 10.48.200.3 --job postgres --format human
```

Example:

```text
2026-06-02 22:07:01  deploy  seq=17  run=8f3a2c12
  reconcile  stop  postgres@10.48.200.3
  $ python3 .../runner.py stop postgres
  postgres@10.48.200.3 | Container postgres  Stopping
  ok  exit=0  1.3s
```

Stream lines in human view include **`job@worker`** when known.

### Examples

```bash
maand logs show --worker 10.48.200.3 --job postgres --phase reconcile --format human
maand logs show --event deploy_skip --format human
maand logs show --run 8f3a2c1d-... --run-dir --worker 10.48.200.3 --format human
grep 'event=command_begin' logs/10.48.200.3.log
grep 'job=cassandra' logs/10.48.200.1.log
```

Use **`--format human`** for debugging; use **`raw`** or grep for automation.
