# maand vs Kubernetes vs Nomad

A side-by-side comparison of orchestration models and where maand fits.

---

## TL;DR

| Feature | maand | Kubernetes | Nomad |
|---------|-------|-----------|-------|
| **Model** | Worker pool + job batching | Pod/container cluster | Task/job scheduler |
| **Scale** | Single cluster, tens of workers | Multi-cluster, thousands+ of nodes | Multi-cloud/DC, 1000s of nodes |
| **Deployment order** | Built-in (`deploy_order`), per-deploy override | StatefulSet ordinals, not restart order | External orchestration required |
| **State management** | Integrated KV store, per-job bootstrap commands | External etcd; no native bootstrap | Consul KV; external orchestration |
| **Configuration** | Go templates + KV, per-worker SSH | ConfigMaps, volumes, declarative | HCL + Consul templates |
| **Bootstrap complexity** | Simple: `command_node_up` per node | Operators / init containers | External (e.g., Consul leader election) |
| **Typical workload size** | 10–100 workers; 1–10 services per worker | 100s–1000s of nodes; 100s of services | 1000s of workers; 100s–1000s of tasks |
| **Operations model** | CLI-driven deploy; visibility-first | Declarative desired state | Job submission + status |
| **Best for** | Stateful clusters (Vault, Cassandra, databases) on managed hardware | Cloud-native, stateless, multi-tenant workloads | Heterogeneous workloads, multi-region batch jobs |

---

## Orchestration model

### maand
- **Worker pool**: fixed set of named hosts (`workers.json`) with labels and capacity.
- **Job placement**: select by label match (e.g., `["cassandra", "vault"]`).
- **Deployment**: explicit CLI commands (`maand deploy`, `maand job start/stop`). Executes **rollout scripts** (Makefile + job commands) on each worker's CLI host in configurable batches.
- **Statefulness**: inherent. Each worker is a distinct host running persistent services. SSH is the transport; data persists on disk.

### Kubernetes
- **Cluster abstraction**: nodes are ephemeral; workloads are portable pods.
- **Placement**: selectors, taints/tolerations, affinity rules match pods to nodes dynamically.
- **Deployment**: declarative manifests (Deployment, StatefulSet, DaemonSet). Desired state → controllers reconcile.
- **Statefulness**: explicit via StatefulSet (ordinal identity), PersistentVolumes, and operators.

### Nomad
- **Task model**: jobs (collections of task groups) submitted to a pool of heterogeneous nodes.
- **Placement**: constraints (CPU, memory, tags, zone). Bin-packing scheduler.
- **Deployment**: job submission with update stragegy (rolling, canary, blue/green). Server-side state management.
- **Statefulness**: possible but not the primary model; more suited to batch and stateless workloads.

---

## Deployment ordering and rolling restarts

### maand — **first-class feature**
```
deploy_order: ["10.0.0.1", "10.0.0.3", "10.0.0.2"]
deploy_parallel_count: 1  (or any N)
```
- Each restart/start batch respects `deploy_order`.
- Override per-deploy in `pre_deploy` via `put_deploy_order(...)` — e.g., leader-last for Raft clusters.
- Example: Vault rolling restart → followers first, active leader last (≤1 leadership transfer).

### Kubernetes — **no native ordering**
- StatefulSet ordinals (pod-0, pod-1, pod-2) exist for **identity**, not restart order.
- Default rolling update: parallel subject to `maxUnavailable`.
- **Workaround**: custom controller/operator (e.g., Vault Operator, etcd-operator) enforces restart order via webhooks or explicit orchestration.

### Nomad — **no native per-allocation ordering**
- Update strategy: `canary`, `blue_green`, rolling with `max_parallel` — but no "node A, then B, then C" control.
- **Workaround**: external orchestration via Nomad API + a coordinating script (semaphores, leader election).

---

## Bootstrap and initialization

### maand — **integrated**
- Job commands (`post_build`, `pre_deploy`, `post_deploy`, `after_allocation_started`).
- Cluster state tracked in **embedded SQLite KV store** with scoped namespaces (`maand/job/<job>`, `vars/job/<job>`, `secrets/job/<job>`).
- Example: Vault bootstrap
  ```python
  # leader initializes cluster, stores unseal keys in KV
  if is_leader: vault_exec("operator init")
  put_job_secret("unseal_key_1", key)
  
  # followers read keys and unseal
  unseal_node(worker_ip)
  ```
  — Single deploy orchestrates the entire cluster.

### Kubernetes — **external**
- No native bootstrap orchestration.
- Patterns:
  - **Init containers**: sequential setup (download configs, check dependencies).
  - **Job resources**: run one-off initialization, then deploy apps.
  - **Operators**: custom controllers watch CRDs and manage complex workflows (Vault Operator, Postgres Operator).
  - **Helm hooks**: `pre-install`, `post-install` — limited control.
- State **external**: leader election via etcd locks, shared KV → etcd, Consul, or a database.

### Nomad — **external**
- Similar to Kubernetes: no native bootstrap.
- Patterns:
  - **Task lifecycle hooks**: `prestart`, `poststart` (limited).
  - **Consul watches**: scripts triggered by Consul state changes.
  - **External orchestrator**: separate tool (custom script, Terraform, scheduler) orchestrates initialization.
- State: Consul KV for elections, shared state.

---

## Configuration and templating

### maand
- **Go templates** rendered at deploy time → per-worker `.tpl` files.
- Access to job KV (global and per-node), job variables, worker metadata.
- Example:
  ```hcl
  # cassandra.yaml.tpl
  listen_address: {{ .WorkerIP }}
  seeds: "{{ get "maand/worker" "cassandra_0" }}"
  ```
- **Per-worker customization**: each node sees its own metadata (`WorkerIP`, `ALLOCATION_IP`).
- Simple, deterministic, no runtime reconfiguration.

### Kubernetes
- **ConfigMaps and Secrets**: store config, mounted as volumes.
- **Helm**: template values into manifests (Go templates or Kustomize).
- **Environment variables**: from ConfigMaps, Secrets, downward API.
- **Re-render on change**: requires redeployment or in-pod reconciliation.
- Stateless by design: config changes trigger new pods.

### Nomad
- **HCL templates** in job specs.
- **Consul template**: renders configs from Consul KV at runtime → daemon-less automatic updates.
- More dynamic than maand; less structured than K8s.

---

## Cluster visibility and operations

### maand
- **CLI-first**: `maand cat` for inspection (jobs, workers, allocations, KV, deployments, health).
- **Real-time logs**: `maand logs`, `maand run_command` for SSH batch execution.
- **Single source of truth**: SQLite catalog (workers, jobs, allocations, versions, KV).
- **Rollout transparency**: `maand deploy --dry-run` shows exactly what will change.
- **Operations model**: explicit, imperative. Deploy is a CLI invocation; auditable and scriptable.

### Kubernetes
- **`kubectl` imperative** or **GitOps declarative** (ArgoCD, Flux).
- **Eventual consistency**: desired state → controllers reconcile.
- **Debugging**: `kubectl logs`, `kubectl describe pod`, `kubectl exec`.
- **Complexity**: learning curve; many abstractions (Deployments, Services, Ingress, RBAC, etc.).
- **Observability**: Prometheus, logging sidecars; no single CLI for everything.

### Nomad
- **Job submission**: `nomad job run` (imperative).
- **Status**: `nomad status`, `nomad logs`, `nomad alloc logs`.
- **Simpler** than K8s but less structured than maand for stateful workloads.

---

## Portability and multi-cluster

### maand
- **Single-cluster only**: one `maand.db` and one worker pool per invocation.
- **Portable clusters**: can restart maand on a different CLI host pointing to the same workers (KV and catalog on disk/workers).
- **Not designed for**: multi-DC failover, multi-region active/active.

### Kubernetes
- **Multi-cluster native**: federation, GitOps (ArgoCD, Flux) across clusters.
- **Node portability**: workload can move across nodes dynamically.
- **Cloud portability**: K8s API is standard across cloud providers.
- **Default choice** for public cloud, SaaS, and hybrid deployments.

### Nomad
- **Multi-region / multi-DC**: designed for it. Federated servers, gossip pool, automatic failover.
- **Heterogeneous hardware**: can schedule on bare-metal, cloud, containers.
- **Better for**: enterprises with on-prem + cloud, edge deployments.

---

## Scaling

### maand
- **Typical**: 10–100 workers, 1–10 services per worker.
- **Scaling model**: add workers to `workers.json`, rebuild, redeploy affected jobs.
- **Overhead**: SQLite catalog, per-worker SSH — starts showing strain above 100s of nodes.
- **Best for**: manageable clusters (small to mid-size deployments, on-prem or co-lo).

### Kubernetes
- **Typical**: 100s–1000s+ of nodes, 100s of services.
- **Autoscaling**: HPA (horizontal), VPA (vertical), cluster autoscaling.
- **Efficiency**: bin-packing, multi-tenant isolation.
- **Overhead**: etcd, apiserver, kubelet (lighter per-node than maand's SSH model).

### Nomad
- **Typical**: 1000s of nodes.
- **Scaling**: built-in federation, gossip, raft consensus.
- **Heterogeneous workloads**: efficient scheduling of diverse task types.

---

## Summary: when to choose what

### Choose **maand** if:
- ✅ Small-to-mid cluster (10–100 workers), stable topology.
- ✅ Stateful workloads (Vault, Cassandra, databases, distributed systems).
- ✅ Need **deployment ordering** and **integrated bootstrap** (e.g., leader-last restarts, cluster initialization in one deploy).
- ✅ Prefer **CLI-driven**, explicit operations with full visibility.
- ✅ Single-cluster; no multi-DC failover needs.
- ✅ Managed/co-located hardware (not cloud auto-scaling).

### Choose **Kubernetes** if:
- ✅ Cloud-native, multi-tenant, stateless workloads.
- ✅ Need multi-cluster, GitOps, infrastructure-as-code.
- ✅ Public cloud (AWS, GCP, Azure).
- ✅ Autoscaling, dynamic node elasticity.
- ✅ Standardized on containers and declarative state.
- ✅ Large teams, ecosystem/community support (Operators, Helm, ArgoCD).

### Choose **Nomad** if:
- ✅ Heterogeneous workloads (containers, VMs, batch jobs, binaries).
- ✅ Multi-region / multi-DC with federation.
- ✅ On-prem + cloud hybrid.
- ✅ Workloads beyond containers (legacy binaries, GPUs, edge).
- ✅ Smaller learning curve than K8s; simpler than maand for state management.

---

## Feature matrix

| Dimension | maand | Kubernetes | Nomad |
|-----------|-------|-----------|-------|
| **Bootstrap orchestration** | ✅ Integrated | ⚠️ Requires operators | ⚠️ External tooling |
| **Deployment ordering** | ✅ Built-in | ❌ No | ❌ External |
| **CLI visibility** | ✅ Single source | ⚠️ Multiple tools | ⚠️ Multiple tools |
| **Stateful workloads** | ✅ Native | ⚠️ Operators + complexity | ⚠️ Not primary model |
| **Multi-cluster** | ❌ No | ✅ Yes | ✅ Yes |
| **Autoscaling** | ❌ Manual | ✅ Yes | ✅ Yes |
| **Multi-DC failover** | ❌ No | ⚠️ Complex | ✅ Yes |
| **Learning curve** | ✅ Shallow | ❌ Steep | ⚠️ Medium |
| **Heterogeneous workloads** | ⚠️ Limited | ⚠️ Container-focused | ✅ Excellent |

---

## Related

- [start/concepts.md](concepts.md) — maand's core model (workers, jobs, allocations)
- [guides/job-commands-tutorial.md](../guides/job-commands-tutorial.md) — bootstrap patterns in maand
- [deployment-sequence.md](../reference/deployment-sequence.md) — maand's deployment ordering
