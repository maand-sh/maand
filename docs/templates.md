# Templates (`.tpl`)

Files ending in **`.tpl`** under **`workspace/jobs/<job>/`** are rendered at **deploy** staging time with Go **`text/template`**. Output drops the `.tpl` suffix (e.g. `config.json.tpl` → `config.json`).

Related: [deploy.md](./deploy.md#staging-and-rsync) · [kv.md](./kv.md) · [job-command.md](./job-command.md)

---

## When rendering runs

During **`maand deploy`**, after job files are copied to `tmp/workers/<ip>/jobs/<job>/` and before rsync to the worker. Each **active allocation** gets its own render pass (per-worker KV and version context).

---

## Template data (dot context)

| Field | Type | Example |
|-------|------|---------|
| `.AllocationID` | string | Stable allocation UUID |
| `.Job` | string | Job name |
| `.CurrentVersion` | string | Running version on this allocation |
| `.NewVersion` | string | Target version from build |
| `.WorkerIP` | string | Worker host |
| `.WorkerID` | string | Worker UUID |
| `.Labels` | `[]string` | Worker labels |
| `.BucketPath` | string | `/opt/worker/<bucket_id>` on worker |
| `.JobPath` | string | `/opt/worker/<bucket_id>/jobs/<job>` |

Equivalent KV:

```text
{{ get "maand/job/api/worker/10.0.0.1" "version" }}
```

---

## Template functions

| Function | Usage | Notes |
|----------|-------|-------|
| `get` | `{{ get "vars/job/api" "cluster_name" }}` | Namespace must be allowed for this job/allocation |
| `getSecret` | `{{ getSecret "db_password" }}` | Shorthand for `secrets/job/<job>` |
| `keys` | `{{ range keys "vars/job/api" }}...{{ end }}` | List keys in a namespace |
| `split` | `{{ split "a,b" "," }}` | String → slice |
| `join` | `{{ join .Labels "," }}` | Slice → string |
| `upper` / `lower` | `{{ upper "hello" }}` | Case conversion |
| `add` / `sub` / `mul` / `div` | `{{ add 1 2 }}` | Integer math |
| `int` | `{{ int "42" }}` | Parse string or pass through int |
| `scrapeConfigs` | `{{ scrapeConfigs }}` | Expands catalog scrape jobs at deploy render (live allocations + ports) |
| `ruleFiles` | `{{ ruleFiles }}` | Lists assembled alert rule paths at deploy render (`rules/<job>/<file>.yaml`) |

Missing keys or disallowed namespaces **panic** at render time (deploy fails for that job).

---

## Example

**`workspace/jobs/api/config.json.tpl`:**

```json
{
  "cluster": "{{ get "vars/job/api" "cluster_name" }}",
  "version": "{{ .NewVersion }}",
  "worker": "{{ .WorkerIP }}",
  "listen_port": "{{ get "maand" "http_port" }}"
}
```

Populate **`vars/job/api`** via **`vars.toml`**, **`pre_deploy`** hooks, or **`post_build`** before deploy.

---

## Prometheus scrape configs

Jobs with `_prometheus/scrape.yaml` or **`scrape.yaml.tpl`** store unexpanded configs in KV at build. Template scrape files are rendered at build with job-level context (`.Job`, `get`, `getSecret` — not `scrapeConfigs` or `.WorkerIP`). **`{{ scrapeConfigs }}`** expands `maand:port/*` placeholders when the prometheus job is staged.

**`workspace/jobs/prometheus/prometheus.yml.tpl`:**

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ['127.0.0.1:9090']
{{ scrapeConfigs }}
```

See [prometheus.md](./prometheus.md) for `_prometheus/` layout, alerts, and runbooks.

---

## Secrets in templates

Use **`getSecret`** for values written by job commands (`put_job_secret`). Do not commit secrets in the workspace.

```text
password = {{ getSecret "db_password" }}
```

---

## Common errors

| Symptom | Fix |
|---------|-----|
| Template panic: namespace not available | Key is outside allowed namespaces for this job — see [kv.md](./kv.md#who-can-read-which-namespaces) |
| Template panic: key not found | Run hook that writes KV before deploy, or add **`vars.toml`** |
| Stale value after hook | Ensure hook runs in **`pre_deploy`** (before stage) or value is in build-time KV |

Debugging: [deploy-debugging.md](./deploy-debugging.md#template--kv-errors-during-stage).
