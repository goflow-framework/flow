# Bun example

This example demonstrates wiring the Bun adapter into `flow.App`, running
`AutoMigrate` (development convenience) and performing basic insert/select
operations.

Run:

```sh
# from the repository root (inside WSL)
go run ./examples/bun_demo
```

You can also generate models with the CLI and then run the migration runner to
apply the generated SQL migrations (see `docs/bun.md`).
