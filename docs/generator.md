# Generator — flags, field syntax and examples

Flow includes a small code generator that can scaffold controllers, models, views
and SQL migrations. The generator is intentionally conservative: it won't
overwrite files unless you pass `--force`, and it supports a compact field
specification syntax for describing model columns.

## CLI flags

- `--force` — overwrite existing files when generating (default: false).
- `--skip-migrations` — do not create migration files when generating scaffolds.
- `--no-views` — do not create view templates when generating scaffolds.
- `--no-i18n` — do not create i18n translation files (app/i18n/en.yaml) when generating scaffolds.
- `--target` — target project root (defaults to current working directory).

These flags are available on the `flow generate` subcommands. The CLI builds
the generator into a temporary binary in integration tests to validate behavior.

## Field specification syntax

Field specifications are provided as variadic arguments to `flow generate model`
and `flow generate scaffold` and follow the form:

  name
  name:type
  name:type,opt1,opt2=val

Examples:

- `title` (defaults to `string`)
- `age:int` (integer column)
- `published_at:datetime` (uses Go `time.Time` and SQL `DATETIME`)
- `price:decimal(10,2),default=0` (decimal with precision/scale and default)
- `name:varchar(50),unique` (varchar with size and unique constraint)
- `email:string,nullable,index` (nullable string and an index)

Supported base types include: `string`/`text`, `int`/`integer`, `int64`,
`bool`/`boolean`, `float`/`float64`, `datetime`/`time`/`timestamp`,
`decimal(precision,scale)` and `varchar(size)` (or `char(size)`).

Options supported after the base type:

- `nullable` — makes the Go field a pointer type and the SQL column nullable.
- `unique` — adds a UNIQUE constraint to the column.
- `index` — generator will add CREATE INDEX statements to the migration.
- `default=<value>` — includes a DEFAULT clause in the migration SQL.
- `ref=<table.column>` or `references=<table.column>` — records a foreign-key reference in the FieldSpec (generator does not currently emit FK constraints automatically).

Notes:

- When a field is declared nullable the generated Go type becomes a pointer
  (e.g. `*string`, `*time.Time`). The generated JSON tag will include
  `omitempty` for nullable fields.
- `decimal` maps to Go `float64` in generated models and the SQL type will
  include the specified precision/scale if provided.

## Examples

Generate a model only (into the current directory):

```bash
# generate a Post model with title:string and published_at:datetime
flow generate model Post title:string published_at:datetime
```

Generate a scaffold (controller, model, views and migrations):

```bash
# scaffold a post resource and create migration files
flow generate scaffold post title:string published_at:datetime
```

Generate a scaffold but skip migrations and do not create views:

```bash
flow generate scaffold post title:string --skip-migrations --no-views
```

Generate a scaffold but do not generate i18n translation files:

```bash
flow generate scaffold post title:string --no-i18n
```

Force overwriting existing files when regenerating:

```bash
flow generate model Post title:string --force
```

## Testing and integration

The repository includes CLI integration tests under `internal/generator` that
build the CLI and run the generator into temporary directories. Those tests
exercise the flags (`--force`, `--skip-migrations`, `--no-views`, `--no-i18n`) and verify
the generated files and migration SQL contents.

## Generator plugins

Flow supports optional third-party generator plugins. A generator plugin
implements the `GeneratorPlugin` contract provided by `pkg/plugins` and may
register itself by calling `plugins.RegisterGenerator` from an `init()`
function. Generator plugins reuse the standard plugin lifecycle (Init,
Mount, Start, Stop) and add a `Generate(projectRoot string, args []string)
([]string, error)` method to run the generator logic and return created
files.

Key points:

- Plugins must provide `Name()` and `Version()`; the framework validates the
  semantic version at registration time and enforces major-version
  compatibility by default via `flow.ValidatePluginVersion`.
- Generator plugins are discovered by the CLI via two small helpers in the
  internal generator package:
  - `internal/generator.ListRegisteredGenerators()` — list registered generator plugin names.
  - `internal/generator.GetRegisteredGenerator(name)` — returns the registered plugin instance.
- To register a generator plugin call `plugins.RegisterGenerator(yourPlugin)`;
  this performs the same validation and collision checks as normal plugins.

Minimal example (plugin package):

```go
package mygen

import (
    "os"
    "path/filepath"

    "github.com/undiegomejia/flow/pkg/plugins"
)

type MyGen struct{}

func (g *MyGen) Name() string { return "mygen" }
func (g *MyGen) Version() string { return "0.0.1" }
func (g *MyGen) Init(app *flow.App) error { return nil }
func (g *MyGen) Mount(app *flow.App) error { return nil }
func (g *MyGen) Middlewares() []flow.Middleware { return nil }
func (g *MyGen) Start(ctx context.Context) error { return nil }
func (g *MyGen) Stop(ctx context.Context) error { return nil }

func (g *MyGen) Generate(projectRoot string, args []string) ([]string, error) {
    out := filepath.Join(projectRoot, "MYGEN.txt")
    if err := os.WriteFile(out, []byte("generated by mygen\n"), 0o644); err != nil {
        return nil, err
    }
    return []string{out}, nil
}

func (g *MyGen) Help() string { return "mygen: example generator" }

func init() {
    _ = plugins.RegisterGenerator(&MyGen{})
}
```

Compatibility notes:

- By default, registration uses `flow.ValidatePluginVersion` which enforces
  that the plugin MAJOR version matches the framework `PluginAPIMajor`.
- For advanced compatibility policies (for example, temporarily allowing a
  range of MAJOR versions during a migration), the framework exposes
  `flow.ValidatePluginVersionRange(v, minMajor, maxMajor)` which accepts a
  version string and a permitted MAJOR range and returns the same sentinel
  errors as the default validator.
