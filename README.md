# Maand

Maand is a workload orchestrator and provisioner that operates without agents, with all states stored in a file.
It is designed to handle a wide variety of workloads within a cluster, automating the execution and management of jobs.

docs : https://maand.sh/latest

## Documentation

- **[Docs index](docs/README.md)** — overview and links
- **[Concepts](docs/concepts.md)** — worker, job, allocation
- **[Jobs & dependencies](docs/jobs-and-dependencies.md)** — job manifest, demands, **version**, deploy **current/new version**, `deployment_seq`
- **[Commands](docs/commands.md)** — full CLI reference
- **[Getting started tutorial](docs/tutorials/getting-started.md)** — step-by-step first deploy

### Command guides

- [Build](docs/build.md) — `maand build`
- [Deploy](docs/deploy.md) — `maand deploy`
- [Health check](docs/health-check.md) — `maand health_check`
- [Job commands](docs/job-command.md) — manifest commands, runtime API
- [Job control](docs/job.md) — `maand job`
- [Run command](docs/run-command.md) — `maand run_command`
- [GC](docs/gc.md) — `maand gc`
- [Info](docs/info.md) — `maand info`

### Tutorials

- [Day-2 operations](docs/tutorials/day-2-operations.md)
- [Job commands](docs/tutorials/job-commands.md)

## Local integration tests

Requires real workers and files under [`assets/`](../assets/) (not in git). See [assets/README.md](../assets/README.md).

21 integration tests cover init, build, deploy (dry-run, rollout, job filter, hooks), job control, health checks, job commands (CLI/KV/secrets), run_command, GC, and info/cat.

```bash
make test-integration
```

# How to build

Maand uses SQLite via CGO (`CGO_ENABLED=1`).

```bash
make build          # produces ./maand
make test           # unit + ./tests packages (same scope as CI)
make test-integration   # real workers; see assets/README.md
```

Or manually:

```bash
export CGO_ENABLED=1
go build -o maand .
```

Add the binary to your `PATH`, then see [docs/tutorials/getting-started.md](docs/tutorials/getting-started.md).