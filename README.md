# Maand

Maand is a workload orchestrator and provisioner that operates without agents, with all states stored in a file.
It is designed to handle a wide variety of workloads within a cluster, automating the execution and management of jobs.

## Documentation

**[Docs index](docs/README.md)** — start here

### Start

- [Overview](docs/start/overview.md) — capabilities and limits
- [Concepts](docs/start/concepts.md) — worker, job, allocation
- [Quickstart](docs/start/quickstart.md) — step-by-step first deploy

### Guides

- [Guides index](docs/guides/README.md)
- [Rolling deploy](docs/guides/rolling-deploy.md)
- [Debugging deploy](docs/guides/debugging-deploy.md)
- [Disable and drain](docs/guides/disable-and-drain.md)
- [Job commands tutorial](docs/guides/job-commands-tutorial.md)
- [Day-2 operations](docs/guides/day-2-ops.md)

### Reference

- [Reference index](docs/reference/README.md)
- [Configuration](docs/reference/configuration.md)
- [Manifest](docs/reference/manifest.md) · [Deployment sequence](docs/reference/deployment-sequence.md)
- [Resources and placement](docs/reference/resources-and-placement.md)
- [Logging](docs/reference/observability/logging.md)
- [KV namespaces](docs/reference/kv/namespaces.md)
- [CLI commands](docs/reference/cli/commands.md)
- [Build](docs/reference/cli/build.md) · [Deploy](docs/reference/cli/deploy.md)

## Local integration tests

Requires real workers and files under [`assets/`](assets/README.md) (not in git). See [assets/README.md](assets/README.md).

```bash
make test-integration
```

## How to build

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

Add the binary to your `PATH`, then see [docs/start/quickstart.md](docs/start/quickstart.md).
