# Templates (`.tpl`)

Files ending in **`.tpl`** under **`workspace/jobs/<job>/`** are rendered at **deploy** staging time with Go **`text/template`**. Output drops the `.tpl` suffix (e.g. `config.json.tpl` → `config.json`).

Related: [deploy.md](cli/deploy.md#staging-and-rsync) · [KV namespaces](kv/namespaces.md) · [cli/job-command.md](cli/job-command.md)

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
| `min` / `max` | `{{ min 128 (max 4 (div $memMB 64)) }}` | Integer bounds |
| `int` | `{{ int (get "vars/job/postgres" "memory") }}` | Parse string or pass through int |
| `scrapeConfigs` | `{{ scrapeConfigs }}` | Expands catalog scrape jobs at deploy render (live allocations + ports) |
| `ruleFiles` | `{{ ruleFiles }}` | Lists assembled alert rule paths at deploy render (`rules/<job>/<file>.yaml`) |

Missing keys or disallowed namespaces **panic** at render time (deploy fails for that job).

Go **`text/template`** built-ins are also available: `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `printf`, `and`, `or`, `not`, `len`, `index`, `range`, `if`, `define`, `template`.

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

### Memory sizing (postgres-style)

**`workspace/jobs/postgres/postgresql.conf.tpl`:**

```text
{{ define "pgMemUnit" -}}
{{- $mb := int . -}}
{{- if ge $mb 1024 -}}
{{- div $mb 1024 -}}GB
{{- else -}}
{{- $mb -}}MB
{{- end -}}
{{- end }}
{{- $memMB := int (get "vars/job/postgres" "memory") -}}
{{- $sharedMB := div $memMB 4 -}}
{{- $cacheMB := div (mul $memMB 3) 4 -}}
{{- $maintMB := min 2048 (div $memMB 16) -}}
{{- $workMB := min 128 (max 4 (div $memMB 64)) -}}
shared_buffers = {{ template "pgMemUnit" $sharedMB }}
effective_cache_size = {{ template "pgMemUnit" $cacheMB }}
maintenance_work_mem = {{ $maintMB }}MB
work_mem = {{ $workMB }}MB
```

Use **`{{ get (printf "maand/worker/%s" .WorkerIP) "postgres_allocation_index" }}`** for per-worker metadata from build.

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

See [prometheus.md](../guides/prometheus.md) for `_prometheus/` layout, alerts, and runbooks.

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
| Template panic: namespace not available | Key is outside allowed namespaces for this job — see [KV persistence](kv/persistence.md#who-can-read-which-namespaces) |
| Template panic: key not found | Run hook that writes KV before deploy, or add **`vars.toml`** |
| Stale value after hook | Ensure hook runs in **`pre_deploy`** (before stage) or value is in build-time KV |

Debugging: [deploy-debugging.md](../guides/debugging-deploy.md#template--kv-errors-during-stage).
