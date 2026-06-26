# Prometheus monitoring (`_prometheus/`)

Jobs opt in to monitoring by adding a **`_prometheus/`** folder under `workspace/jobs/<job>/`. Nothing goes in `manifest.json` for scrape, alerts, or runbooks.

Maand **build** discovers these files, expands scrape targets, and aggregates a catalog into KV (`maand/prometheus/*`). The **prometheus job** renders its config at deploy; other jobs never need edits when a new job is added.

Related: [build.md](../reference/cli/build.md) · [deploy.md](../reference/cli/deploy.md) · [templates.md](../reference/templates.md) · [health-check.md](../reference/cli/health-check.md)

---

## Job layout

```text
workspace/jobs/api/
├── manifest.json
├── _prometheus/
│   ├── scrape.yaml          # []scrape_config — native Prometheus shape
│   ├── scrape.yaml.tpl      # optional; rendered at build (job-level templates)
│   ├── alerts/
│   │   ├── slo.yaml
│   │   └── errors.yaml
│   └── runbooks/            # optional
│       └── ApiDown.md
└── ...
```

| Path | Purpose |
|------|---------|
| `_prometheus/scrape.yaml` | YAML **array** of Prometheus `scrape_config` items |
| `_prometheus/scrape.yaml.tpl` | Optional template rendered at **build** (see below) |
| `_prometheus/alerts/*.yaml` | Alerting rule groups (multiple files OK) |
| `_prometheus/runbooks/*.md` | Runbooks linked from alert annotations |

**Not synced to workers** — `_prometheus/` is excluded from deploy rsync (like `_modules`). Content is stored in `job_files` at build for the catalog and deploy-time runbook HTML.

---

## `_prometheus/scrape.yaml`

Root is a YAML array. Each element uses the same fields as [Prometheus scrape_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config).

```yaml
- job_name: api
  metrics_path: /metrics
  scheme: http
  static_configs:
    - targets:
        - maand:port/api_metrics_port
      labels:
        maand_job: api
```

Use **`maand:job`** instead of repeating the maand job name for `job_name` and label values. Build stores the resolved name in **`maand/job/<job>/job_name`** KV:

```yaml
- job_name: maand:job
  static_configs:
    - targets:
        - maand:port/api_metrics_port
      labels:
        maand_job: maand:job
```

```bash
maand cat kv get maand/job/api job_name
```

Multiple scrape jobs per maand job are allowed (separate array entries). **`job_name` must be unique across the whole bucket** (after `maand:job` expansion).

### Dynamic targets (`maand:port/*`)

Use placeholders where worker IPs and assigned ports are not known at commit time:

| Target | Expands to |
|--------|------------|
| `maand:port/<port_key>` | Every **active allocation** → `<worker_ip>:<assigned_port>` |
| `maand:port/_implicit` | `<job>_metrics_port` → `metrics_port` → sole `*_metrics_port` |
| `maand:port/<port_key>/<worker_ip>` | Single allocation on that worker |
| `maand:job` | Maand job folder name (for `job_name` and `static_configs.labels` values) |

`<port_key>` must exist in `manifest.resources.ports`. Literal `host:port` targets pass through unchanged.

Service discovery configs (`kubernetes_sd_configs`, etc.) are **not** supported in v1 — use `static_configs` with maand placeholders.

### `scrape.yaml.tpl` (optional)

Use **`scrape.yaml.tpl`** instead of **`scrape.yaml`** when you need Go templates at build time. **Do not define both** — build fails like `prometheus.yml` / `prometheus.yml.tpl`.

Rendered during **`maand build`** (after job KV is synced). Supports the same template functions as deploy `.tpl` files except **`scrapeConfigs`** (not available here). Per-allocation fields such as **`{{ .WorkerIP }}`** are **not** available — use **`maand:port/*`** for targets.

| Field | Available |
|-------|-----------|
| `{{ .Job }}` | yes |
| `{{ .NewVersion }}`, `{{ .CurrentVersion }}` | yes (manifest version) |
| `{{ get "maand/job/<job>" "…" }}` | yes |
| `{{ get "vars/job/<job>" "…" }}` | yes |
| `{{ getSecret "…" }}` | yes |
| `{{ .WorkerIP }}`, worker certs | no |

```yaml
# _prometheus/scrape.yaml.tpl
- job_name: {{ .Job }}
  metrics_path: {{ get "vars/job/api" "metrics_path" }}
  static_configs:
    - targets:
        - maand:port/api_metrics_port
      labels:
        maand_job: {{ .Job }}
```

Build stores the **rendered YAML** (with `maand:port/*` and `maand:job` still unexpanded) in KV. Deploy expands placeholders when rendering `{{ scrapeConfigs }}`.

---

## `_prometheus/alerts/`

Rules stay **authored per job**. Build validates YAML structure, required fields, optional runbook references, and **unique alert names across the whole bucket** (recording rules use `record:` instead of `alert:`).

```yaml
# _prometheus/alerts/availability.yaml
groups:
  - name: api
    rules:
      - alert: ApiDown
        expr: up{job="api"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API {{ $labels.instance }} down"
          runbook: ApiDown
      - record: api:up:sum
        expr: sum(up{job="api"})
```

Each rule must define **`expr`** and exactly one of **`alert`** or **`record`**.

If `annotations.runbook` is set, `_prometheus/runbooks/<name>.md` must exist. During **`maand deploy --jobs prometheus`**, maand removes **`runbook`** and adds **`runbook_url`** pointing at the rendered console page unless you set **`runbook_url`** yourself.

---

## Build aggregation

During **`maand build`**, after allocations and ports are known:

| KV key | Content |
|--------|---------|
| `maand/prometheus/scrape_configs` | JSON array of unexpanded scrape configs (`maand:port/*` placeholders preserved) |
| `maand/prometheus/scrape/<job>` | Per-job unexpanded scrape configs |
| `maand/prometheus/scrape_jobs` | Comma-separated maand jobs with scrape.yaml |
| `maand/prometheus/alert_files` | JSON index of alert file paths |
| `maand/prometheus/runbooks/<job>/<slug>` | Runbook metadata |

Build validates scrape structure, port references, alert rule shape, runbook links, and **global uniqueness** of Prometheus `job_name` and alert rule names.

Inspect after build:

```bash
maand cat prometheus
maand cat prometheus get api scrape.yaml
maand cat prometheus scrape
maand cat kv get maand/prometheus scrape_jobs
maand cat kv get maand/prometheus scrape_configs
```

---

## Prometheus job config

### Option A — `prometheus.yml.tpl` (recommended)

Combine hardcoded scrape jobs with the build catalog:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ['127.0.0.1:9090']
{{ scrapeConfigs }}

{{ ruleFiles }}
```

`{{ scrapeConfigs }}` expands `maand:port/*` placeholders using **live allocations and assigned ports** at deploy render time, then injects the YAML fragment. When the catalog or allocation topology changes, redeploy the prometheus job so the rendered hash updates.

`{{ ruleFiles }}` lists every assembled alert file as `rules/<maand_job>/<file>.yaml` (paths relative to `prometheus.yml`). Prefer this over globs.

If you use a glob instead, Prometheus only supports Go **`filepath.Glob`** syntax — **not** `**`. Maand lays out rules as `rules/<job>/<file>.yaml`, so use one `*` per directory level:

```yaml
rule_files:
  - rules/*/*.yaml
```

Inside Docker, mount the rules tree and use absolute paths if you prefer:

```yaml
rule_files:
  - /etc/prometheus/rules/*/*.yaml
```

```yaml
# docker-compose.yml.tpl (prometheus job)
volumes:
  - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
  - ./rules:/etc/prometheus/rules:ro
```

### Option B — static `prometheus.yml`

If **`prometheus.yml`** exists and **`prometheus.yml.tpl` does not**, the file is copied as-is with no maand injection. Use for fully manual setups.

**Do not define both** — build fails with `ErrInvalidJob`.

### Alert rules at deploy

When a job ships `prometheus.yml` or `prometheus.yml.tpl`, deploy **assembles** alert YAML from every job's `_prometheus/alerts/` into `rules/<maand_job>/` under the prometheus staging tree.

---

## Operator workflow

```bash
# 1. Add _prometheus/ to app jobs; declare metrics ports in manifest
maand build

# 2. Deploy application jobs
maand deploy --jobs api,...

# 3. Redeploy prometheus to pick up new scrape targets and rules
maand deploy --jobs prometheus
```

Prometheus should run with **`--web.enable-lifecycle`** so config reload works after deploy.

---

## Runbooks

During **`maand deploy`**, when staging the **prometheus** job, maand renders every catalog runbook to HTML under **`consoles/runbooks/<job>/<slug>.html`** (plus **`consoles/runbooks/index.html`** and **`consoles/runbooks/style.css`**). Source markdown stays in **`job_files`** from build; it is not rsynced from workspace.

Mount **`./consoles`** into Prometheus console templates (runbooks are generated inside that tree):

```yaml
# docker-compose.yml.tpl (prometheus job)
volumes:
  - ./consoles:/etc/prometheus/consoles:z
  - ./console_libraries:/etc/prometheus/console_libraries:z
  # ...
command:
  - '--web.console.templates=/etc/prometheus/consoles'
```

| URL (Prometheus UI) | File on worker |
|---------------------|----------------|
| `/consoles/runbooks/` | `consoles/runbooks/index.html` |
| `/consoles/runbooks/<job>/<slug>.html` | `consoles/runbooks/<job>/<slug>.html` |

Link from alert annotations with **`runbook`** in workspace source only — deploy removes **`runbook`** and sets **`runbook_url`**:

```yaml
# workspace source
annotations:
  runbook: container_restarting

# deployed rules (prometheus worker)
annotations:
  runbook_url: http://10.48.198.160:9090/consoles/runbooks/node_agent/container_restarting.html
```

Override **`runbook_url`** in the alert YAML when you need a different base URL.

---

## Participation

| Job has | Scraped | Rules loaded | Runbooks |
|---------|---------|--------------|----------|
| `_prometheus/scrape.yaml` only | yes | no | no |
| `_prometheus/alerts/` only | no | yes | no |
| both | yes | yes | optional |
| neither | no | no | no |

---

## Example: node_exporter

**`manifest.json`** — runs on any worker that has the **`node_exporter`** label (job name); add `"selectors": ["worker"]` when sharing a generic worker pool:

```json
{
  "resources": {
    "ports": { "node_exporter_metrics_port": {} }
  }
}
```

Or on shared workers:

```json
{
  "selectors": ["worker"],
  "resources": {
    "ports": { "node_exporter_metrics_port": {} }
  }
}
```

**`_prometheus/scrape.yaml`**

```yaml
- job_name: node_exporter
  metrics_path: /metrics
  static_configs:
    - targets:
        - maand:port/node_exporter_metrics_port
      labels:
        maand_job: node_exporter
```

---

## Related commands

| Command | Role |
|---------|------|
| `maand build` | Aggregate `_prometheus/` → KV |
| `maand deploy --jobs prometheus` | Render config, assemble rules, generate runbook HTML |
| `maand cat kv get maand/prometheus …` | Inspect catalog KV |
| `maand cat prometheus` | List jobs with `_prometheus/` (scrape, alerts, runbooks) |
| `maand cat prometheus get <job> <path>` | Print one `_prometheus/` file (catalog or workspace; `scrape` → `scrape.yaml`) |
| `maand cat prometheus scrape [--jobs …]` | Preview expanded scrape configs at deploy time |
