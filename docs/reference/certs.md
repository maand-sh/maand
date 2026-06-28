# TLS certificates

Maand generates a bucket **certificate authority (CA)** at init and **per-job TLS certificates** at build time. Material is stored in the **KV store**, then copied to workers during **deploy** under each job’s `certs/` directory.

Related: [configuration.md](configuration.md) · [build.md](cli/build.md#certificates) · [deploy.md](cli/deploy.md#staging-and-rsync) · [KV persistence](kv/persistence.md)

---

## Overview

```text
maand init
  └── secrets/ca.crt, ca.key          (bucket CA, long-lived)

maand build
  └── BuildCerts
        ├── signs leaf certs with CA
        └── KV: maand/job/<job>/worker/<ip>/certs/<name>.{crt,key}
        └── KV: maand/worker/certs/ca.crt   (CA PEM for all workers)

maand deploy
  └── tmp/workers/<ip>/jobs/<job>/certs/
        ├── <name>.crt, <name>.key
        └── ca.crt
  └── rsync → /opt/worker/<bucket_id>/jobs/<job>/certs/
```

| Layer | Location | Purpose |
|-------|----------|---------|
| **Bucket CA** | `<bucket>/secrets/ca.crt`, `ca.key` | Signs all job leaf certificates |
| **Build / KV** | `maand/job/<job>/worker/<ip>/certs/*` | Canonical PEM storage between build and deploy |
| **Worker disk** | `/opt/worker/<bucket_id>/jobs/<job>/certs/` | What your app/Makefile reads at runtime |

Certificates are **not** checked into the workspace. Declare them in **`manifest.json`**; maand generates keys and PEMs.

---

## Bucket CA (`maand init`)

On first **`maand init`**, maand creates:

| File | Description |
|------|-------------|
| `secrets/ca.crt` | Self-signed CA certificate |
| `secrets/ca.key` | CA private key (PKCS#1 RSA) |

Defaults:

- **Subject CN** = bucket ID
- **TTL** = 10 years (`10 × 365` days)
- **Key size** = 4096-bit RSA (1024 in test mode)

If only one of `ca.crt` / `ca.key` exists, init fails — both must be present or both absent.

The CA PEM is copied into KV at **`maand/worker/certs/ca.crt`** on every build so deploy can place it on each worker.

### Rotating the CA

Replacing **`secrets/ca.crt`** or **`secrets/ca.key`** changes the CA file hash. The next **`maand build`** regenerates **all** job leaf certificates signed by the new CA.

```bash
# Backup old CA, install new pair under secrets/, then:
maand build
maand deploy    # push new leaf certs + ca.crt to workers
```

Plan a rolling **`maand deploy`** so every allocation picks up the new trust chain. Applications must trust the new `ca.crt`.

---

## Job manifest (`certs`)

Add a **`certs`** map to **`workspace/jobs/<job>/manifest.json`**. Each key is a cert **name** (used for filenames and KV keys).

```json
{
  "selectors": ["worker"],
  "certs": {
    "tls": {
      "pkcs8": false,
      "one": false,
      "subject": { "common_name": "api.internal" }
    },
    "client": {
      "pkcs8": true,
      "one": true,
      "subject": { "common_name": "api-cluster" }
    }
  }
}
```

| Field | Default | Meaning |
|-------|---------|---------|
| `subject.common_name` | (required) | X.509 subject CN |
| `pkcs8` | `false` | `true` → private key PEM type **`PRIVATE KEY`** (PKCS#8); `false` → **`RSA PRIVATE KEY`** (PKCS#1) |
| `one` | `false` | `true` → **one shared** cert/key pair copied to every allocation; `false` → **per-worker** cert (SANs include that worker’s IP) |

### Per-worker vs shared (`one`)

| `one` | Behavior |
|-------|----------|
| `false` (default) | Each allocation gets its own key pair. Leaf cert **IP SANs**: `127.0.0.1` + that worker’s IP. |
| `true` | One cert is generated (using the first worker in the allocation set) and the **same** `.crt`/`.key` PEMs are written to every worker namespace. SANs include `127.0.0.1` and **all** allocated worker IPs. Use for cluster-wide identities (e.g. mutual TLS where all nodes present the same cert). |

Certs are generated for **all non-removed allocations** (active and disabled). Disabled allocations still receive KV material on build; deploy stages it without starting the process.

### Removing certs

Delete the **`certs`** block from the manifest (or remove a name). The next **`maand build`** purges matching **`certs/*`** keys from affected allocation namespaces.

---

## Bucket configuration (`maand.conf`)

Leaf certificate lifetime and renewal are controlled in **`maand.conf`** at the bucket root:

```toml
ssh_user = "agent"
ssh_key = "worker.key"
certs_ttl = 60
certs_renewal_buffer = 10
```

| Field | Default in code | Purpose |
|-------|-----------------|--------|
| `certs_ttl` | `60` (days) if unset or `0` | Validity period for **newly generated** leaf certificates |
| `certs_renewal_buffer` | `0` if unset | Regenerate a stored cert when within this many days of `NotAfter` (`0` = only after expiry) |

### Auto-rotation at build time

Rotation is **not** a background daemon. **`maand build`** checks each stored leaf cert and regenerates when:

1. **Missing** — no `certs/<name>.crt` or `.key` in KV  
2. **CA changed** — `secrets/ca.crt` hash differs from last promoted build  
3. **Manifest changed** — `certs` section hash differs (`build_certs` namespace)  
4. **Expiring soon** — `now >= NotAfter - certs_renewal_buffer` (see below)

If none apply, existing PEMs are **reused** (repeat builds without config changes do not churn certificates).

**Renewal buffer math**

```text
regenerate when:  now >= cert.NotAfter - (certs_renewal_buffer days)
                 (or now >= NotAfter when buffer is 0)
```

| `certs_renewal_buffer` | Effect |
|------------------------|--------|
| `10` | Regenerate when within **10 days** of expiry (recommended for scheduled builds) |
| `0` | Regenerate only **after** `NotAfter` has passed |

If **`certs_renewal_buffer` ≥ `certs_ttl`**, every leaf cert is always in the renewal window (regenerated on each **`maand build`**).

Set both values explicitly in `maand.conf`. Example from [configuration.md](configuration.md):

```toml
certs_ttl = 60
certs_renewal_buffer = 10
```

### Operational rotation workflow

Schedule regular **`maand build`** (cron/CI). When the renewal buffer triggers, build writes new PEMs to KV. **`maand deploy`** then rsyncs them to workers.

```bash
maand build
maand deploy --dry-run   # optional: see if rollout needed
maand deploy
```

After deploy, apps read updated files from:

```text
/opt/worker/<bucket_id>/jobs/<job>/certs/<name>.crt
/opt/worker/<bucket_id>/jobs/<job>/certs/<name>.key
/opt/worker/<bucket_id>/jobs/<job>/certs/ca.crt
```

Restart or reload the job if it caches TLS material (e.g. **`make restart`** on the next deploy wave, or your own hook).

### Prometheus metrics (optional)

When a **prometheus** job ships `prometheus.yml` or `prometheus.yml.tpl`, **`maand deploy`** (after commit) pushes certificate expiry gauges to Prometheus remote write (`/api/v1/write`). Push runs **only at deploy**, not at **`maand build`**. Failures are **best-effort** — logged, deploy still succeeds. Transient failures (503, other 5xx, network errors) are retried up to 5 times with backoff (2s–15s) because Prometheus may not accept remote write immediately after rollout.

| Metric | Meaning |
|--------|---------|
| `maand_cert_not_after_seconds` | Unix timestamp of certificate `NotAfter` |
| `maand_cert_expiring` | `1` when within `certs_renewal_buffer` of expiry (same window as build renewal) |
| `maand_cert_expired` | `1` when past `NotAfter` |

Labels: `scope` (`ca` or `job`), `job`, `worker`, `cert`, `common_name`, `status`.

**Remote write URL** — auto-discovered from the **prometheus** job in the workspace: first non-removed allocation IP and **`prometheus_port_http`** → `http://<worker>:<port>/api/v1/write`. No `maand.conf` setting; if there is no prometheus job with server config, push is skipped.

Prometheus must run with **`--web.enable-remote-write-receiver`**. Deploy also writes embedded cert alert rules to **`rules/maand/certs.yaml`** on the prometheus worker. See [prometheus.md](../guides/prometheus.md#certificate-alerts).

---

## Where certificates live

### KV (after build)

| Namespace | Keys |
|-----------|------|
| `maand/worker` | `certs/ca.crt` |
| `maand/job/<job>/worker/<ip>` | `certs/<name>.crt`, `certs/<name>.key` |

Inspect:

```bash
maand cat certs --jobs api
maand cat certs --workers 10.0.0.1
maand cat kv get maand/job/api/worker/10.0.0.1 certs/tls.crt   # raw PEM (truncated in list)
```

Job commands and templates may read PEMs from the allocation namespace (prefer **file paths on the worker** for large certs in production).

### Worker filesystem (after deploy)

During deploy staging, **`updateCerts`** writes:

```text
tmp/workers/<ip>/jobs/<job>/certs/
├── <name>.crt    # mode 0644
├── <name>.key    # mode 0600
└── ca.crt
```

The tree is rsynced with the rest of the job. **`BuildJobAllocationVariables`** runs after **`BuildCerts`** so cert sync does not delete allocation metadata keys (`*_allocation_index`, `peer_workers`).

---

## Using certs in your job

### Makefile / application

Point your service at the deploy path (values also available as template fields `.JobPath` and `.BucketPath`):

```makefile
TLS_CERT := $(CURDIR)/certs/tls.crt
TLS_KEY  := $(CURDIR)/certs/tls.key
TLS_CA   := $(CURDIR)/certs/ca.crt
```

`CURDIR` is the job directory on the worker when **`make start`** runs via `runner.py`.

### Templates

Prefer filesystem paths in rendered config:

```json
{
  "tls_cert": "{{ .JobPath }}/certs/tls.crt",
  "tls_key": "{{ .JobPath }}/certs/tls.key",
  "tls_ca": "{{ .JobPath }}/certs/ca.crt"
}
```

See [templates.md](templates.md).

### Job commands

Read from the runtime KV API or use paths after deploy. Namespace: `maand/job/<job>/worker/<worker_ip>`.

---

## Examples

### Single service TLS (per worker)

```json
{
  "selectors": ["worker"],
  "certs": {
    "tls": {
      "subject": { "common_name": "api.example.com" }
    }
  }
}
```

Each worker gets a distinct key; cert includes that host’s IP and `127.0.0.1`.

### Shared cluster cert

```json
{
  "certs": {
    "internal": {
      "one": true,
      "pkcs8": true,
      "subject": { "common_name": "mycluster" }
    }
  }
}
```

All allocations share identical PEMs in KV and on disk.

### Stricter renewal

```toml
# maand.conf
certs_ttl = 90
certs_renewal_buffer = 30
```

Build regenerates leaf certs in the last 30 days of their 90-day life. Pair with weekly **`maand build && maand deploy`**.

---

## Inspecting certificates (`maand cat certs`)

List the bucket CA and every job leaf cert stored in KV with expiration and renewal status:

```bash
maand cat certs
maand cat certs --jobs api,postgres
maand cat certs --workers 10.0.0.1
```

| Column | Meaning |
|--------|---------|
| `scope` | `ca` (bucket CA in `secrets/`) or `job` (leaf cert in KV) |
| `job` / `worker` | Allocation (blank for CA) |
| `cert` | Cert name from manifest (`tls`, `client`, …) or `ca` |
| `common_name` | X.509 subject CN |
| `not_after` | Certificate expiry (UTC) |
| `days_left` | Whole days until expiry (negative if expired) |
| `status` | `ok`, `expiring` (same window as **`maand build`** renewal), `expired`, or `invalid` |

---

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| No `certs/` on worker | Job not deployed, or manifest has no `certs` block |
| Cert unchanged across builds | Still valid and outside renewal buffer; expected |
| All certs regenerated | CA file changed, manifest `certs` edited, or cert entered renewal window |
| Build OK, worker has old cert | Run **`maand deploy`** after build |
| `ErrInvalidManifest` on cert | Invalid `subject` JSON |
| App rejects peer cert | Worker still using old `ca.crt` — redeploy after CA rotation; check **`maand cat certs`** |
| Disabled allocation missing certs in KV | Run **`maand build`** — disabled rows still get cert material |

---

## Related

- [configuration.md](configuration.md) — `maand.conf` fields
- [build.md](cli/build.md) — build pipeline step `BuildCerts`
- [KV persistence](kv/persistence.md) — persistence and namespaces
- [disable and drain](../guides/disable-and-drain.md) — disabled allocations still receive build certs
