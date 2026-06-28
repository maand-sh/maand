# Prometheus monitoring (`_prometheus/`)

Jobs opt in to monitoring by adding a **`_prometheus/`** folder under `workspace/jobs/<job>/`. Each subfolder or file is optional: scrape config, alerts, runbooks, and dashboards are independent.

Maand **build** validates `_prometheus/` content, stores files in `job_files`, and writes **scrape configs only** to KV (`maand/prometheus/*`). The **prometheus job** renders its config at deploy; alerts, runbooks, and dashboards are assembled from `job_files`.

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
│   ├── runbooks/            # optional
│   │   └── ApiDown.md
│   └── dashboards/          # optional Prometheus console pages
│       └── overview.html
└── ...
```

| Path | Purpose |
|------|---------|
| `_prometheus/scrape.yaml` | YAML **array** of Prometheus `scrape_config` items |
| `_prometheus/scrape.yaml.tpl` | Optional template rendered at **build** (see below) |
| `_prometheus/alerts/*.yaml` | Alerting rule groups (multiple files OK) |
| `_prometheus/runbooks/*.md` | Runbooks linked from alert annotations |
| `_prometheus/dashboards/**` | Prometheus console HTML (and assets) copied at deploy |

**Not synced to workers** — `_prometheus/` is excluded from deploy rsync (like `_modules`). Build copies content into **`job_files`**. Deploy assembles alerts, runbooks, dashboards, and expanded scrape config when staging the **prometheus** job.

---

## Prerequisites

| Requirement | Effect |
|-------------|--------|
| **Prometheus server job** in workspace (`prometheus.yml` or `prometheus.yml.tpl`) | Enables `_prometheus/` **validation** and scrape **KV** at build; enables deploy-time assembly (rules, consoles, `{{ scrapeConfigs }}`) |
| No prometheus server job | `_prometheus/` on app jobs is **ignored** at build (invalid scrape.yaml + scrape.yaml.tpl pairs are not rejected); scrape KV is cleared |
| Active allocations (for `maand:port/*` scrape targets) | Job appears in aggregate **`scrape_jobs`** / **`scrape_configs`**; without active allocations the job is omitted from aggregate KV but **`scrape/<job>`** per-job KV is still written |
| Literal `host:port` scrape targets | No active allocations required — job always included in scrape catalog |

Nothing in `_prometheus/` is required on any job. A job may ship scrape only, alerts only, runbooks only, dashboards only, or any combination.

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

Rendered during **`maand build`** (after job KV is synced). Supports **`get`**, **`getSecret`**, **`keys`** — not **`getOptional`**, **`scrapeConfigs`**, or per-allocation fields such as **`{{ .WorkerIP }}`**. Use **`maand:port/*`** for targets.

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

## Build (`maand build`)

Runs only when a **prometheus server job** exists in the workspace.

### Validation (all jobs)

- Scrape YAML shape, port references, forbidden SD configs
- Alert YAML shape, global unique alert names, runbook file references
- Mutual exclusion: `scrape.yaml` vs `scrape.yaml.tpl`; `prometheus.yml` vs `prometheus.yml.tpl`

All `_prometheus/` files are stored in **`job_files`**. Alerts, runbooks, and dashboards are **not** written to KV.

### Scrape KV (`maand/prometheus/*`)

| KV key | Content |
|--------|---------|
| `scrape_configs` | JSON array of unexpanded configs for jobs in **`scrape_jobs`** |
| `scrape/<job>` | Per-job unexpanded scrape configs (always written when scrape file exists) |
| `scrape_jobs` | Comma-separated jobs included in the aggregate catalog |
| `scrape_jobs_length` | Count of `scrape_jobs` |

**Aggregate inclusion:** a job with `maand:port/*` targets is listed in `scrape_jobs` / `scrape_configs` only when it has at least one **active** allocation. Jobs with only literal targets are always included.

At deploy, jobs with zero expanded targets are **skipped** (not an error).

Inspect after build:

```bash
maand cat prometheus
maand cat prometheus get api scrape.yaml
maand cat prometheus scrape
maand cat kv get maand/prometheus scrape_jobs
maand cat kv get maand/prometheus scrape/<job>
```

---

## Deploy assembly

When staging the prometheus job (see [deploy.md](../reference/cli/deploy.md#prometheus-job-staging)):

1. **Alert rules** — copy each `_prometheus/alerts/*.yaml` from `job_files` to `rules/<maand_job>/`; inject **`runbook_url`** from **`runbook`** annotation; add **`rules/maand/certs.yaml`** when server config exists
2. **Runbooks** — render markdown to `consoles/runbooks/<job>/<slug>.html` (+ index, CSS)
3. **Dashboards** — copy `consoles/dashboards/<job>/<path>` preserving subdirectories (+ index, CSS)
4. **Config** — render `prometheus.yml.tpl` with **`{{ scrapeConfigs }}`** and **`{{ ruleFiles }}`**
5. **Cert metrics** — after deploy commit, best-effort remote write of cert expiry gauges (not at build)

Mount on the prometheus container:

```yaml
volumes:
  - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
  - ./rules:/etc/prometheus/rules:ro
  - ./consoles:/etc/prometheus/consoles:z
  - ./maand:/etc/prometheus/consoles/maand:z   # optional workspace custom pages
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

Deploy **assembles** alert YAML from every job's `_prometheus/alerts/` in `job_files` into `rules/<maand_job>/` under the prometheus staging tree (see [Deploy assembly](#deploy-assembly) above).

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

### Certificate alerts

When the **prometheus** job ships server config, deploy assembles **`rules/maand/certs.yaml`** alongside per-job alert files. These rules fire on pushed cert metrics (`maand_cert_expiring`, `maand_cert_expired`, etc.) — see [certs.md](../reference/certs.md#prometheus-metrics-optional).

After **`maand deploy`**, maand pushes current cert expiry gauges via Prometheus remote write to:

```text
http://<prometheus-allocation>:<prometheus_port_http>/api/v1/write
```

Push runs **only at deploy** (not **`maand build`**). Discovery uses the **prometheus** job from the workspace (first non-removed allocation). If that job has no server config or no allocations, push is skipped.

Requires **`--web.enable-remote-write-receiver`** on Prometheus (in addition to lifecycle):

```yaml
command:
  - '--web.enable-lifecycle'
  - '--web.enable-remote-write-receiver'
```

When **`secrets/job/prometheus`** defines **`admin_username`** and **`admin_password`**, cert metric remote write uses HTTP Basic auth (same credentials as the Prometheus UI). If the web UI is protected, both secrets must be set or deploy logs a cert-metrics push error.

## Runbooks

During **`maand deploy`**, when staging the **prometheus** job, maand renders every catalog runbook to HTML under **`consoles/runbooks/<job>/<slug>.html`** (plus **`consoles/runbooks/index.html`** and **`consoles/runbooks/style.css`**). Source markdown stays in **`job_files`** from build; it is not rsynced from workspace.

Runbooks are **not** written under workspace **`maand/`** — that folder is for optional custom console pages you check in (for example `maand/workers.html`).

Mount deploy-generated runbooks and optional custom pages separately:

```yaml
# docker-compose.yml.tpl (prometheus job)
volumes:
  - ./consoles:/etc/prometheus/consoles:z          # deploy writes consoles/runbooks/ here
  - ./maand:/etc/prometheus/consoles/maand:z        # optional custom pages from workspace
  - ./maand_libraries:/etc/prometheus/console_libraries:z
  # ...
command:
  - '--web.console.templates=/etc/prometheus/consoles'
  - '--web.console.libraries=/etc/prometheus/console_libraries'
```

| URL (Prometheus UI) | File on worker |
|---------------------|----------------|
| `/consoles/runbooks/` | `consoles/runbooks/index.html` |
| `/consoles/runbooks/<job>/<slug>.html` | `consoles/runbooks/<job>/<slug>.html` |
| `/consoles/maand/workers.html` | `maand/workers.html` (workspace, not deploy-generated) |

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

## Dashboards

During **`maand deploy`**, when staging the **prometheus** job, maand copies every file under **`_prometheus/dashboards/`** to **`consoles/dashboards/<job>/<path>`**, preserving subdirectories. Source files stay in **`job_files`** from build; they are not rsynced from workspace.

Optional **`_prometheus/dashboards/.dashboardignore`** (gitignore-style, one pattern per line) omits matching files from **`dashboards/index.html`** only. Ignored files are still copied to the worker and reachable by direct URL. Deploy reads the latest **`.dashboardignore` from workspace** (run **`maand build`** if you rely on catalog-only workflows elsewhere).

```gitignore
# partials and fragments — not listed on index
_partial.html
partials/**
**/worker_detail.html
```

| URL (Prometheus UI) | File on worker |
|---------------------|----------------|
| `/consoles/dashboards/` | `consoles/dashboards/index.html` |
| `/consoles/dashboards/<job>/overview.html` | `consoles/dashboards/<job>/overview.html` |
| `/consoles/dashboards/<job>/slo/latency.html` | `consoles/dashboards/<job>/slo/latency.html` |

The same **`./consoles:/etc/prometheus/consoles`** mount serves runbooks and dashboards.

---

## Participation

| Job has | Scraped | Rules loaded | Runbooks | Dashboards |
|---------|---------|--------------|----------|------------|
| `_prometheus/scrape.yaml` only | yes | no | no | no |
| `_prometheus/alerts/` only | no | yes | no | no |
| `_prometheus/dashboards/` only | no | no | no | yes |
| both scrape + alerts | yes | yes | optional | optional |
| neither | no | no | no | no |

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
| `maand build` | Validate `_prometheus/`; store in `job_files`; write scrape KV (when prometheus server job exists) |
| `maand deploy --jobs prometheus` | Assemble rules, runbooks, dashboards; render config; cert metric push |
| `maand cat kv get maand/prometheus scrape_jobs` | Inspect scrape catalog |
| `maand cat prometheus` | List jobs with `_prometheus/` content (from `job_files` / workspace) |
| `maand cat prometheus get <job> <path>` | Print one `_prometheus/` file |
| `maand cat prometheus scrape [--jobs …]` | Preview expanded scrape configs (same logic as deploy) |
