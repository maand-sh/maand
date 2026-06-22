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
maand run_command "sudo reboot" --workers 10.0.0.3

# 3. Wait for SSH (manual or scripted sleep)

# 4. Re-enable — remove 10.0.0.3 from disabled.json
maand build && maand deploy
```

Disabled allocations **keep** deploy artifacts and KV; after reboot, **`deploy`** **starts** jobs again on re-enable.

---

## Pattern B — rolling reboot across a fleet

```bash
WORKERS="10.0.0.1 10.0.0.2 10.0.0.3"

for ip in $WORKERS; do
  echo "=== drain $ip ==="
  # Edit disabled.json to add only this worker under "workers"
  maand build && maand deploy

  maand run_command "sudo reboot" --workers "$ip"
  sleep 120   # tune for your boot time

  # Remove worker from disabled.json
  maand build && maand deploy
  maand health_check --wait
done
```

Automate **`disabled.json`** edits with a script or config management tool.

---

## Pattern C — parallel SSH only (no catalog drain)

For workers where a hard reboot without drain is acceptable:

```bash
maand run_command "sudo reboot" --workers 10.0.0.1,10.0.0.2 --concurrency 1
```

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
