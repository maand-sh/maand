# KV store

Maand configuration and secrets live in an in-memory **KV store** backed by SQLite (`key_value` table). **Build**, **deploy**, **job commands**, and **templates** read and write KV.

| Document | Use when |
|----------|----------|
| [namespaces.md](./namespaces.md) | Which keys exist, how to set/read them, examples (includes `maand/prometheus` scrape catalog) |
| [persistence.md](./persistence.md) | When writes commit, purge rules, access control |

Inspect from the CLI:

```bash
maand cat kv
maand cat kv --jobs api --active
maand cat kv get maand/job/api version
maand cat kv get --reveal secrets/job/api db_password
```

Related: [cli/job-command.md](../cli/job-command.md) · [templates.md](../templates.md) · [cli/build.md](../cli/build.md)
