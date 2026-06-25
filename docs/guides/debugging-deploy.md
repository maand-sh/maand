# Debugging deployment issues

Structured checklist for **`maand deploy`** failures, skipped jobs, partial rollouts, and worker sync problems. Start with **dry-run** and **cat** commands before SSHing to workers.

---

## Quick diagnostic flow

```text
1. maand deploy --dry-run [-b]     # would deploy run? which jobs?
2. maand cat deployments [--jobs J]     # per-allocation rollout state
3. maand cat allocations [--jobs J]
4. maand cat jobs                  # deployment_seq, disabled flag
5. maand build && maand deploy     # refresh plan hashes after workspace edit
6. Worker: /opt/worker/<bucket_id>/worker.json, jobs/<job>/logs/
```

```bash
maand info
maand deploy --dry-run
maand cat deployments --jobs api
maand cat allocations --jobs api
maand cat kv --jobs api
```

---

## Dry-run first

```bash
maand deploy --dry-run
maand deploy -b -n              # build + dry-run
maand deploy --dry-run --jobs api,vault
maand deploy --dry-run --force  # preview forced restart
```

Dry-run **stages** locally and refreshes plan hashes; it does **not** change workers or commit hash promotion. It may not simulate **stop** of removed/disabled allocations (see [deploy.md](../reference/cli/deploy.md)).

Interpret output:

| Message | Meaning |
|---------|---------|
| `deployment required` | At least one job needs rollout |
| `no deployment required` | All active allocations promoted (hashes + versions match) |
| `deploy required` per job | That job will stage/rsync/restart |
| `skip` / `already promoted` | **`JobNeedsRollout`** false for that job |

---

## Read allocation hash state

```bash
maand cat deployments
maand cat deployments --jobs api --active
maand cat deployments --workers 10.0.0.1
```

| Rollout | Meaning | Typical action |
|---------|---------|----------------|
| `new` | Never promoted | First deploy → **start** |
| `restart` | Staged content or version differs from promoted | **restart** on deploy |
| `promoted` | In sync | Skipped unless **`--force`** |
| `health_failed` | Health check marked allocation (`--update-hash`) | Fix health, then **`deploy`** or **`deploy --force`** |
| `disabled` | Allocation disabled; stopped; catalog current | Re-enable via [disabled.md](disable-and-drain.md) |
| `disabled_restart` | Disabled; catalog has pending content/version | Deploy updates plan; still no restart until active |
| `removed` | Soft-deleted allocation | **`deploy`** then **`gc`** |

Columns **`current_version`** / **`new_version`**: version-only rollout when hashes match but versions differ.

---

## Common symptoms

### `deploy: skip job "..." (already promoted on all allocations)`

| Cause | Fix |
|-------|-----|
| No workspace change since last successful deploy | Expected; edit job or use **`--force`** |
| Edited workspace but only ran deploy | Run **`maand build`** then **`maand deploy`** |
| Version bump without content change | **`build`** sets **`new_version`**; deploy should restart — check **`cat deployments`** for version mismatch |
| TLS cert expired or expiring soon | **`maand cat certs`** — then **`maand build && maand deploy`** — [certs.md](../reference/certs.md) |
| Job fully **disabled** | No active allocations; enable or use **`--jobs`** on active jobs only |

### Job not in deploy wave at all

```bash
maand cat jobs                    # job exists? deployment_seq?
maand cat allocations --jobs J    # any active rows?
```

| Cause | Fix |
|-------|-----|
| No matching workers | Fix **`selectors`** vs **`worker_labels`**; check **`workers.json`** |
| Job filtered out | **`deploy --jobs`** list omits it |
| Lower **`deployment_seq`** jobs failing | Fix earlier wave; deploy processes seq 0 first |
| **`pre_deploy`** failed | Check job-command logs; job skipped from staging this run |

### `pre_deploy` / `post_deploy` / hook failure

Hooks run on the **CLI host** (Python/Bun). Failures return **`deploy failed`** with the job name.

```bash
maand cat job_commands --jobs api
maand job_command api command_migrate --verbose   # reproduce cli event
```

| Event | On failure |
|-------|------------|
| **`pre_deploy`** | Job not staged this run; others continue |
| **`post_deploy`** | Job deploy fails; earlier jobs in the run may already be promoted |
| **`job_control`** | Entire job deploy path fails |

KV written during hooks commits with deploy on success; rolls back if deploy aborts before commit.

### SSH / rsync errors

```bash
# From maand host
ssh -i secrets/worker.key agent@10.0.0.1 true
```

| Symptom | Check |
|---------|--------|
| Connection refused / timeout | Firewall, worker down, wrong IP |
| Permission denied | **`secrets/worker.key`** authorized on worker |
| rsync / sudo errors | **`maand.conf`**: `use_sudo`, `ssh_user`; worker needs **`rsync`**, **`make`**, **`python3`**, **`timeout`** |
| Host prerequisites | CLI needs **`bash`**, **`ssh`**, **`rsync`**, **`python3`** (and **`bun`** if `.ts`/`.js` commands exist) |

Deploy logs **`deploy: removed worker X unreachable, assuming dead`** for off-catalog removed allocations — usually safe to ignore.

### `health check failed`

```bash
maand health_check --jobs api --wait --verbose
```

| Cause | Fix |
|-------|-----|
| Probe not ready yet | Increase wait; fix startup order / **`deployment_seq`** |
| Command health failed | Fix script; **`maand health_check --update-hash`** then **`deploy`** |
| Wrong port in manifest | **`maand cat job_ports --jobs api`** |

### `worker.json` / `update_seq` mismatch (`maand job` only)

```bash
maand job status api    # fails sync check
maand deploy            # refreshes worker.json on workers
```

Deploy does not require this check; **`maand job`** does.

### Partial deploy (some jobs succeeded, others failed)

Deploy **commits** successful jobs; failed jobs stay unpromoted.

```bash
maand cat deployments        # promoted vs restart per job
maand deploy            # retries only jobs still needing rollout
```

Fix the failing job, re-run **`maand deploy`** — promoted jobs are skipped.

### Template / KV errors during stage

```bash
maand cat kv --jobs api
maand cat kv get vars/job/api mykey
```

| Error | Fix |
|-------|-----|
| `get` / `getSecret` missing key | Run hook that writes KV (**`post_build`**, **`pre_deploy`**) or **`maand job_command`** |
| Template panic | Allowed namespaces only — see [templates.md](../reference/templates.md) |

### Disabled / re-enable surprises

| Symptom | Fix |
|---------|-----|
| Job stopped after disable | Expected; see [disabled.md](disable-and-drain.md) |
| Re-enabled job not starting | Run **`maand build`** after clearing **`disabled.json`**, then **`deploy`** |
| KV missing on disabled job | Should be retained; if gone, check whether allocations were **removed** not disabled |
| **`cat kv --jobs J`** empty | Job may be fully **removed** (no non-removed allocations) |

### Removed allocation / GC

```bash
maand build && maand deploy && maand gc
maand cat allocations --jobs api
```

Removed rows: hash cleared on deploy; worker **`jobs/<job>/`** tree deleted on **gc**. See [gc.md](../reference/cli/gc.md).

---

## Worker-side inspection

Paths on worker (replace **`<bucket_id>`** from **`maand info`**):

```text
/opt/worker/<bucket_id>/worker.json
/opt/worker/<bucket_id>/jobs.json
/opt/worker/<bucket_id>/jobs/<job>/Makefile
/opt/worker/<bucket_id>/jobs/<job>/data/
/opt/worker/<bucket_id>/jobs/<job>/logs/
/opt/worker/<bucket_id>/bin/runner.py
```

```bash
maand run_command "cat /opt/worker/<bucket_id>/worker.json" --workers 10.0.0.1
maand run_command "ls -la /opt/worker/<bucket_id>/jobs/api/" --workers 10.0.0.1
```

Compare **`update_seq`** in **`worker.json`** with **`maand info`**.

---

## Staging directory (maand host)

During deploy, inspect rendered trees before rsync:

```text
tmp/workers/<worker_ip>/jobs/<job>/
tmp/workers/<worker_ip>/worker.json
```

If deploy fails mid-run, this directory is removed when the command exits.

---

## Logging

| Source | Location |
|--------|----------|
| Deploy command | stderr from **`maand deploy`** (skip lines, SSH errors, health) |
| Job-command runtime API | **`logs/`** under bucket root |
| Worker job logs | **`jobs/<job>/logs/`** on worker (job-defined) |

---

## Command reference for debugging

| Goal | Command |
|------|---------|
| Bucket summary | `maand info` |
| Plan deploy | `maand deploy --dry-run` |
| Hash / version state | `maand cat deployments` |
| Allocation flags | `maand cat allocations` |
| Deploy order | `maand cat jobs` |
| KV for templates/hooks | `maand cat kv --jobs <job>` |
| Ports | `maand cat job_ports --jobs <job>` |
| Reproduce hook | `maand job_command <job> <cmd> --verbose` |
| Health only | `maand health_check --jobs <job> --wait --verbose` |
| Force reroll | `maand deploy --force --jobs <job>` |

---

## Escalation checklist

1. **`maand build`** succeeded after last workspace edit?
2. **`maand cat deployments`**: expected rollout state per worker?
3. **`maand deploy --dry-run`**: job listed as required?
4. **`deployment_seq`**: blocked by an earlier job?
5. **`disabled.json`**: unintended drain?
6. **`pre_deploy`** / **`post_deploy`** logs clean?
7. Worker prerequisites and SSH from CLI host?
8. Health probes passing with **`--wait`**?

---

## Related

- [deploy.md](../reference/cli/deploy.md) — pipeline and failure table
- [disabled.md](disable-and-drain.md) — disable and re-enable
- [rolling-deploy](rolling-deploy.md) — **`update_parallel_count`** and reboot patterns
- [health-check.md](../reference/cli/health-check.md) — **`--update-hash`**
- [job-command-api.md](../reference/job-command-api.md) — hook debugging
- [tutorials/day-2-operations.md](day-2-ops.md) — operations checklist
