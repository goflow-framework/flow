# Admin UI Generator — Design Contract

Purpose

Design contract for an "admin" generator that scaffolds a basic admin UI for a resource. This document defines the inputs, outputs, file layout, CLI flags, generated code contracts, error modes and tests for a minimal, low-risk implementation.

Scope (MVP)

- Input: resource name + field list (same format as existing generators, e.g. `title:string published_at:datetime`).
- Output:
  - Admin controller with CRUD actions (Index/List, Show, New, Create, Edit, Update, Delete).
  - Server-rendered view templates for list/show/new/edit and an `admin` layout.
  - Route registration stub (function that mounts admin routes onto the App).
  - Model update (if model exists, do nothing unless --force; optionally generate model + migration when `--auth` or when model missing).
  - Migration: create table migration with given fields (unless --skip-migrations).
  - Optional auth scaffolding: `User` model, login/logout controller, `RequireLogin` and `RequireRole` middleware skeletons when `--auth` is passed.

Design goals

- Low friction: generated code uses existing `pkg/flow` APIs (controllers, views, sessions).
- Non-opinionated about frontend stack: default server-rendered templates; `--react` emits a small stub and README to integrate a preferred toolchain.
- Safe: generator should refuse to overwrite files unless `--force` is provided.
- Extensible: place hooks and TODO comments in templates where developers are likely to customize behavior.

Generator API (internal)

- New generator function signature (internal/generator):

  ```go
  func GenerateAdminWithOptions(projectRoot, name string, opts GenOptions, fields ...string) ([]string, error)
  ```

- CLI wiring (cmd/flow): `flow generate admin NAME [fields...]` with flags:
  - `--target` (project root) — already supported globally in `generate` commands
  - `--force` (overwrite existing)
  - `--skip-migrations` (do not create migrations)
  - `--no-views` (skip server views)
  - `--auth` (generate auth scaffolding)
  - `--react` (emit a minimal React stub under `web/admin` and brief README)

Files & Conventions (example for resource `posts`)

- Controller: `app/controllers/admin/posts_controller.go`
  - Package `controllers`
  - Type `PostsAdminController` embedding `*flow.Controller`.
  - Exported helper `MountAdminPostsRoutes(app *flow.App)` or `func (c *PostsAdminController) Mount(app *flow.App)`; prefer a free function `MountAdminRoutes(app *flow.App)` to be called by `main` or registered by a plugin.

- Views: `app/views/admin/posts/index.html`, `show.html`, `new.html`, `edit.html`
  - Use an admin layout at `app/views/admin/layouts/admin.html` with a simple navigation and flash display.

- Migration: `db/migrate/<timestamp>_create_posts.up.sql` and `.down.sql` generated using the generator migration template, including indexes and nullability based on field specs.

- Model: If `app/models/post.go` doesn't exist, generator will create a Bun-friendly model (same as `GenerateModelWithOptions`) and optionally include convenience methods.

- Auth (when `--auth` present):
  - `app/models/user.go` + migration with fields `email:string`, `password_hash:string`, `role:string`.
  - `app/controllers/auth_controller.go` with `Login` and `Logout` methods using `flow.Session()`.
  - `pkg/middleware/auth.go` (or `app/middleware`) with `RequireLogin` and `RequireRole(role string)` returning `flow.Middleware`.
  - The generated admin controllers include a TODO to call `RequireLogin` middleware before sensitive actions; generator may add `app.Use(RequireLogin)` in route-mounting function if `--auth` is used.

Mounting contract

- The generator will produce a `MountAdminRoutes(app *flow.App)` function in the generated code that calls `app.SetRouter` or registers handlers on an existing router. Preferred pattern (non-invasive):

  - Generate a function that accepts a `*flow.App` and registers routes using `app.Router()` or by returning a `router` that the main app can compose.

  Example:

  ```go
  package admin

  import "github.com/dministrator/flow/pkg/flow"

  func MountAdminRoutes(app *flow.App) {
      r := flow.NewRouter(app)
      r.Get("/admin/posts", postsController.Index)
      r.Get("/admin/posts/:id", postsController.Show)
      // etc.
      app.SetRouter(r.Handler())
  }
  ```

  - Another approach: generator can output a small package-level `Init` that registers a plugin via `plugins.Register(...)` — this is optional and behind a `--plugin` flag.

Error modes & handling

- Existing files: generator will refuse to overwrite any file unless `opts.Force` is true. It will return a clear error listing conflicting files. CLI prints `created <path>` for created files and `skipped <path>` for existing files.

- Bad field specs: reuse `ParseFields` from the existing generator; return a friendly error including the bad token and expected syntax.

- Partial failure: generator should ensure atomicity of output list: created files are returned in the `[]string` and migrations are created last. If migration creation fails after controller/model generation, the generator returns an error but does not attempt to rollback created files — document this limitation and encourage `--force` + manual cleanup for retries.

Testing strategy

- Unit tests (internal/generator): generate admin into a temporary directory and assert the expected files exist. Use compile-and-run test pattern from `gen_compile_test.go` to ensure generated code compiles.

- Integration: add a small CI smoke test that runs `go run` against an example app with generated admin to ensure routes mount and a minimal `curl` hits the index endpoint (optional for MVP).

Extensibility & hooks

- Provide template placeholders and comments where developers should customize behavior (field renderers, custom widgets).

- Empahsize that custom field types should be handled by developers; generator will render TODOs in templates to help developers adapt.

- Consider adding `--plugin` flag to emit admin as a compile-time plugin under `pkg/plugins/admin` that calls `plugins.Register()`.

Delivery & next steps

1. Implement generator templates and `GenerateAdminWithOptions` (1–2 days). Keep initial auth generation disabled behind `--auth` (implement as extra 0.5–1 day if desired).
2. Wire CLI `genAdminCmd` into `cmd/flow` (0.5 day).
3. Add unit test(s) and compile test (0.5–1 day).
4. Optional: implement `--react` flag for a minimal frontend stub (0.5–1 day).

Immediate action I will take next (if you confirm):
- Add templates and implementation for `GenerateAdminWithOptions` (controller + views + route mount) and wire `gen admin` CLI command (MVP with no auth). I will run generator unit tests after implementing.

Questions for you (to finalize design choices):
- Do you prefer generated admin code to auto-register as a plugin (via `plugins.Register()`), or prefer explicit mounting in `main` (safer, less magic)?
- Should the generator create `app/models/<model>.go` if a model does not exist, or should it always assume the model exists and only create controller/views/migrations?

Please confirm those two design choices and I will implement the MVP generator templates and CLI wiring next.