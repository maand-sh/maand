# Rolling worker reboot (host OS)

Maand has no built-in **`reboot`** command. Use **disable → stop → reboot → re-enable**.

Related: [disable-and-drain.md](./disable-and-drain.md) · [rolling-deploy.md](./rolling-deploy.md)

---

## Pattern A — disable worker, reboot, re-enable

```bash
# 1. Drain all jobs on the host
cat > workspace/disabled.json <<'EOF'
{ "workers": ["10.0.0.3"] }
EOF
maand build && maand deploy

# 2. Reboot (SSH from maand host)
maand run_command "sudo shutdown -r now" --workers 10.0.0.3

# 3. Wait for SSH (manual or scripted sleep)

# 4. Re-enable — remove 10.0.0.3 from disabled.json
maand build && maand deploy

# 5. Optional: verify all jobs after re-enable
maand health_check --jobs api,worker --wait
```

Disabled allocations **keep** deploy artifacts and KV; after reboot, **`deploy`** **starts** jobs again on re-enable.

---

## Pattern B — rolling reboot across a fleet

```bash
maand run_command "sudo shutdown -r now" --workers 10.0.0.1,10.0.0.2,10.0.0.3 --concurrency 1

# Optional: verify all jobs after the rolling reboot
maand health_check --wait
```

No shell loop is required for the reboot command itself; **`maand run_command`** batches workers and honors **`--concurrency`**.

If you want strict **disable -> deploy -> reboot -> re-enable -> deploy** per worker, you still need per-worker `disabled.json` edits (scripted or manual).

---

## Pattern C — parallel SSH only (no catalog drain)

For workers where a hard reboot without drain is acceptable:

```bash
maand run_command "sudo shutdown -r now" --workers 10.0.0.1,10.0.0.2 --concurrency 1
```

You can add **`--health_check`** to **`maand run_command`** to run job health checks after each command batch:

```bash
maand run_command "systemctl restart docker" --workers 10.0.0.1,10.0.0.2 --concurrency 1 --health_check
```

For **reboot** specifically, prefer explicit **`maand health_check --wait`** after the host returns, because health checks triggered immediately after `shutdown -r now` may run while the worker is still booting.

**`--concurrency 1`** reboots one host at a time. After reboot, processes may be down until **`maand job start`** or **`maand deploy`**. Prefer Pattern A for production.

---

## After reboot: sync check

```bash
maand job status api --allocations 10.0.0.3
```

If **`worker.json` / `update_seq` mismatch**, run **`maand deploy`** before **`maand job`**.

---

## Related

- [disable-and-drain.md](./disable-and-drain.md)
- [../reference/cli/run-command.md](../reference/cli/run-command.md)
- [../reference/cli/health-check.md](../reference/cli/health-check.md)
