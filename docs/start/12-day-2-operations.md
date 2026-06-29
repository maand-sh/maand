# 12. Day-2 operations

After your first deploy, these commands cover most ongoing work.

---

## Manual job control

Requires a prior deploy (workers have current `runner.py` and `update_seq`).

```bash
maand job status api
maand job status api --allocations 10.0.0.1

maand job stop api --allocations 10.0.0.2
maand job start api --allocations 10.0.0.2
maand job restart api --health_check

maand job run api --target reload
maand job run api --target logs    # if Makefile defines logs
```

These invoke **`runner.py`** → **`make <target>`** on workers over SSH.

Reference: [job.md](../reference/cli/job.md).

---

## Ad-hoc remote commands

```bash
maand run_command "uptime"
maand run_command "df -h /opt/worker" --workers 10.0.0.1
maand run_command "make -s -C jobs/api logs" --workers 10.0.0.1
maand run_command "docker ps" --labels web --concurrency 2
```

Working directory on workers: `/opt/worker/<bucket_id>/`.

Reference: [run-command.md](../reference/cli/run-command.md).

---

## Drain for maintenance

1. Edit **`workspace/disabled.json`**
2. **`maand build`**
3. **`maand deploy`** — stops disabled allocations, does not restart them

Re-enable: remove entry, build, deploy.

Guide: [disable-and-drain.md](../guides/disable-and-drain.md).

---

## Remove jobs or workers

1. Delete from `workspace/` (remove job dir or host from `workers.json`)
2. **`maand build`** — marks allocations **`removed`**
3. **`maand deploy`** — stop, remove deployed files; keeps `data/` and `logs/`
4. **`maand gc`** — purge catalog rows and delete worker runtime trees

Guide: [day-2-ops.md](../guides/day-2-ops.md).

---

## Upgrades and rollouts

```bash
# edit manifest version or job files
maand build
maand deploy --dry-run
maand deploy --jobs api
```

Rolling behavior: [rolling-deploy.md](../guides/rolling-deploy.md).

Worker OS reboot: [worker-reboot.md](../guides/worker-reboot.md).

---

## Debugging checklist

| Symptom | Check |
|---------|-------|
| Deploy skipped everything | `maand deploy --dry-run`, `maand cat deployments` |
| One allocation failed | `maand logs show --worker <ip> --job <job> --format human` |
| SSH errors | `ssh -i secrets/worker.key user@host`, `maand.conf` |
| Stale manual job cmd | `maand deploy` to refresh `update_seq` |
| App unhealthy | `maand health_check --jobs <job> --verbose` |

Guide: [debugging-deploy.md](../guides/debugging-deploy.md).

---

## Inspect state (quick reference)

```bash
maand info
maand cat workers
maand cat jobs
maand cat allocations
maand cat deployments
maand cat certs --jobs api
maand cat kv --jobs api
```

---

## You have completed the tour

| Next step | Document |
|-----------|----------|
| Hands-on practice | [quickstart.md](./quickstart.md) |
| Full glossary | [concepts.md](./concepts.md) |
| Feature checklist | [overview.md](./overview.md) |
| All CLI flags | [commands.md](../reference/cli/commands.md) |
| Deep hooks | [job-commands-tutorial.md](../guides/job-commands-tutorial.md) |
