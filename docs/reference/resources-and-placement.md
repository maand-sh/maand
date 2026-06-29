# Job resources, bucket overrides, and environment selectors

Maand uses three layers to place jobs and validate capacity:

1. **`manifest.json`** — declares **min/max** memory and CPU for a job (portable bounds checked into git).
2. **`bucket.jobs*.conf`** — sets the **actual reservation** (`current_memory_mb` / `current_cpu_mhz`) for the current environment; values must stay within manifest min/max.
3. **`workers.json`** — declares **host capacity** and **labels** used for placement and validation.

**Environment file naming:** `maand.conf` → **`job_config_selector`** picks the override file. Empty selector → **`bucket.jobs.conf`**. Non-empty (e.g. `"prod"`) → **`bucket.jobs.prod.conf`**. General pattern: **`bucket.jobs.<selector>.conf`**.

After edits, run **`maand build`**. Build fails if a reservation exceeds manifest bounds, or if active allocations on a worker need more memory/CPU than that worker declares.

Related: [configuration.md](configuration.md) · [concepts.md](../start/concepts.md#allocation) · [build.md](cli/build.md#workspaceworkersjson)

---

## How the pieces fit together

```text
manifest.json          bucket.jobs.prod.conf       workers.json
  min / max bounds  →    memory / cpu override  →   capacity + labels
        │                         │                        │
        └──────────── build ──────┴────────────────────────┘
                              │
                    allocations (label match)
                    ValidateWorkerResources
```

| Layer | File | What it controls |
|-------|------|------------------|
| Job bounds | `workspace/jobs/<job>/manifest.json` | Allowed memory/CPU range (`min` / `max`) |
| Job reservation | `workspace/bucket.jobs[.<env>].conf` | **Current** memory/CPU charged against workers (must be within bounds) |
| Worker capacity | `workspace/workers.json` | Available memory/CPU; **labels** for placement |

Disabled allocations are excluded from capacity validation (`removed=0`, `disabled=0` only).

---

## Memory and CPU in `manifest.json`

Declare optional limits under `resources`:

```json
{
  "selectors": ["worker", "prod"],
  "resources": {
    "memory": { "min": "256 mb", "max": "2 gb" },
    "cpu": { "min": "500 mhz", "max": "2000 mhz" }
  }
}
```

### Rules

| Rule | Behavior |
|------|----------|
| Omitted `min` / `max` | Treated as `0` (no bound on that side) |
| `min` set, `max` omitted or `0` | `max` defaults to `min` |
| `min` > `max` | Build fails (`ErrInvalidManifest`) |
| Units | Memory: `mb`, `gb`, `tb` (case-insensitive). CPU: `mhz`, `ghz`, `thz` |
| Plain numbers | `"512"` → 512 MB or 512 MHz |

If both memory and CPU are omitted (all zeros), the job does not participate in worker resource validation unless a bucket override sets `memory` or `cpu`.

### What build stores

| DB / KV field | Source |
|---------------|--------|
| `min_memory_mb`, `max_memory_mb` | Manifest `resources.memory` |
| `min_cpu_mhz`, `max_cpu_mhz` | Manifest `resources.cpu` |
| `current_memory_mb`, `current_cpu_mhz` | Manifest max, or bucket override (below) |

KV namespace **`maand/job/<job>`** exposes `min_memory_mb`, `max_memory_mb`, `memory`, `min_cpu_mhz`, `max_cpu_mhz`, `cpu` for templates and commands.

---

## Bucket-level override (`bucket.jobs.conf`)

Per-job **reservations** live in TOML under **`workspace/`**. They set what build stores as **`current_memory_mb`** and **`current_cpu_mhz`** — the values summed per worker during validation.

```toml
[api]
memory = "512 mb"
cpu = "1500 mhz"

[worker]
memory = "128 mb"
```

### Bounds checking

The override must fall **inside** the manifest min/max:

```text
manifest min ≤ bucket.jobs.conf value ≤ manifest max
```

Examples:

| Manifest | `bucket.jobs.conf` | Result |
|----------|-------------------|--------|
| `min: 128`, `max: 512` | `memory = "192 mb"` | OK — current = 192 |
| `min: 128`, `max: 256` | `memory = "512 mb"` | Build fails — above max |
| no memory in manifest | `memory = "256 mb"` | OK — min and max set to 256 |

The same rules apply to **`cpu`**.

### Environment-specific files (`job_config_selector`)

Set **`job_config_selector`** in **`maand.conf`** to pick which override file build reads:

```toml
# maand.conf
job_config_selector = "prod"
```

| `job_config_selector` | File read |
|-----------------------|-----------|
| `""` (default) | `workspace/bucket.jobs.conf` |
| `"prod"` | `workspace/bucket.jobs.prod.conf` |
| `"staging"` | `workspace/bucket.jobs.staging.conf` |

Example layout for two environments on one bucket checkout:

```text
workspace/
├── bucket.jobs.conf          # default / dev
├── bucket.jobs.staging.conf
└── bucket.jobs.prod.conf
```

```toml
# bucket.jobs.staging.conf
[api]
memory = "256 mb"
cpu = "500 mhz"
```

```toml
# bucket.jobs.prod.conf
[api]
memory = "1 gb"
cpu = "2000 mhz"
```

Switch environments by changing **`job_config_selector`** in `maand.conf`, then **`maand build`**. Job manifests stay the same; only the reservation file changes.

Override keys are also copied to KV namespace **`vars/bucket/job/<job>`** (along with any other keys in that job’s TOML section).

---

## Worker capacity (`workers.json`)

Workers must declare capacity when any **active** allocation on that host reserves memory or CPU:

```json
[
  {
    "host": "10.0.0.1",
    "labels": ["worker", "prod"],
    "memory": "8192 mb",
    "cpu": "4000 mhz"
  },
  {
    "host": "10.0.0.2",
    "labels": ["worker", "staging"],
    "memory": "4096 mb",
    "cpu": "2000 mhz"
  }
]
```

Build sums **`current_memory_mb`** and **`current_cpu_mhz`** of all active allocations on each worker and compares to **`available_memory_mb`** / **`available_cpu_mhz`**.

Typical failure:

```text
worker_ip 10.0.0.1, available memory is 4096.00 MB, required memory is 5120.00 MB
```

Fix by raising worker capacity, lowering a job reservation in `bucket.jobs.conf`, or moving a job to another worker (selectors).

To discover capacity from live hosts:

```bash
maand worker_facts --dry-run
maand worker_facts --build
```

See [worker-facts.md](cli/worker-facts.md).

---

## Selectors for different environments

**Selectors** are worker **labels**. Build creates an allocation for job × worker only when **every** job selector appears on the worker. When `selectors` is omitted from the manifest, the **job name** is the selector. The label **`worker`** is added automatically to every host.

### Pattern: one job, environment labels

**Workers**

```json
[
  { "host": "10.0.0.10", "labels": ["worker", "prod"], "memory": "16 gb", "cpu": "8 ghz" },
  { "host": "10.0.0.20", "labels": ["worker", "staging"], "memory": "8 gb", "cpu": "4 ghz" }
]
```

**Job** (`workspace/jobs/api/manifest.json`)

```json
{
  "selectors": ["worker", "prod"],
  "resources": {
    "memory": { "min": "256 mb", "max": "4 gb" },
    "cpu": { "min": "200 mhz", "max": "4000 mhz" }
  }
}
```

`api` allocates only on **`10.0.0.10`**. Staging workers never receive it.

For a staging copy, add `workspace/jobs/api-staging/` (or reuse the same tree) with:

```json
{ "selectors": ["worker", "staging"], "resources": { ... } }
```

### Pattern: shared manifest bounds, per-env reservations

Keep one manifest with wide bounds in git:

```json
"resources": {
  "memory": { "min": "128 mb", "max": "4 gb" },
  "cpu": { "min": "100 mhz", "max": "8000 mhz" }
}
```

Tune actual reservations per environment in the matching `bucket.jobs.<env>.conf` without editing the job directory.

### Pattern: one bucket per environment

Many teams use **separate bucket directories** (or separate `maand.conf` / selector) per environment instead of mixing prod and staging workers in one `workers.json`. Selectors then separate job types (`gpu`, `arm`) within that environment.

### Inspecting placement

```bash
maand build
maand cat allocations
maand cat allocations --jobs api
```

Each row is a (job, worker) pair. If a job has no rows, no worker matched all selectors (manifest `selectors`, or the job name when selectors are omitted).

---

## End-to-end example

**Goal:** `api` runs on prod with 1 GB RAM; `api-staging` on staging with 256 MB.

1. **`workers.json`** — prod and staging hosts with labels and capacity (see above).

2. **`jobs/api/manifest.json`**

   ```json
   {
     "selectors": ["worker", "prod"],
     "resources": {
       "memory": { "min": "256 mb", "max": "2 gb" },
       "cpu": { "min": "500 mhz", "max": "2000 mhz" }
     }
   }
   ```

3. **`jobs/api-staging/manifest.json`** — same resources, `"selectors": ["worker", "staging"]`.

4. **`maand.conf`** — `job_config_selector = "prod"`.

5. **`bucket.jobs.prod.conf`**

   ```toml
   [api]
   memory = "1 gb"
   cpu = "1500 mhz"
   ```

6. Switch to staging: set `job_config_selector = "staging"`, use **`bucket.jobs.staging.conf`**:

   ```toml
   [api-staging]
   memory = "256 mb"
   cpu = "500 mhz"
   ```

7. Build and verify:

   ```bash
   maand build
   maand cat allocations
   maand cat kv --jobs api,api-staging
   ```

   KV for each job includes `memory`, `min_memory_mb`, `max_memory_mb`, and the same for CPU.

---

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| `ErrInsufficientResource` on build | Sum of job reservations on a worker exceeds `workers.json` capacity |
| `worker_ip … must specify memory in workers.json` | Job reserves memory but worker has no `memory` field |
| `ErrUnsupportedResourceConfiguration` | `bucket.jobs.conf` memory/CPU outside manifest min/max |
| Job has no allocations | No worker has all required selector labels |
| Override ignored | Wrong `job_config_selector` or typo in TOML job section name (must match job directory name) |
| Validation ignores a job | Allocation is **disabled** or **removed** |

---

## Related

- [configuration.md](configuration.md) — `maand.conf`, `bucket.jobs.conf`, `job_config_selector`
- [build.md](cli/build.md#validateworkerresources) — build pipeline and validation step
- [concepts.md](../start/concepts.md#allocation) — how label matching creates allocations
- [KV persistence](kv/persistence.md) — `maand/job/<job>` and `vars/bucket/job/<job>` namespaces
