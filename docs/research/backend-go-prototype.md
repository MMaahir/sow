# backend: go prototype

Run on 2026-11-10 for ticket #104, in a throwaway Go module outside the repo. It compares the options for the Go backend's HTTP framework, OpenAPI document, database library, migration tool, logger and OpenTelemetry setup, and proves the chosen set against Postgres, an OTLP receiver and orval. ADR 0014 records the decision.

Every version below was the latest release on the Go module proxy, npm or Docker Hub when it was checked on 2026-11-10.

## Versions proven together

| Module or tool | Version | Released |
|---|---|---|
| Go | 1.27.4 | |
| `github.com/danielgtaylor/huma/v2` | 2.40.0 | 2026-10-27 |
| `github.com/jackc/pgx/v5` | 5.12.0 | 2026-10-31 |
| `github.com/pressly/goose/v3` | 3.30.0 | 2026-11-10 |
| sqlc (binary, through mise's aqua backend) | 1.31.1 | 2026-04-22 |
| `go.opentelemetry.io/otel`, `/sdk`, `/sdk/metric`, `otlptracehttp`, `otlpmetrichttp` | 1.47.0 | 2026-10-02 |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | 0.72.0 | 2026-10-02 |
| `go.opentelemetry.io/otel/semconv` | v1.43.0 package, inside otel 1.47.0 | |
| golangci-lint | 2.15.0 | 2026-10-16 |
| Postgres image | `postgres:18` (18.6) | |
| OTLP receiver image | `otel/opentelemetry-collector-contrib:0.165.0` | |
| Runtime image | `gcr.io/distroless/static-debian13:nonroot` | |
| orval, with `typescript` 7.0.2 | 8.47.0 | 2026-11-07 |

Also checked and compared, but not chosen: `github.com/oapi-codegen/oapi-codegen/v2` 2.8.2 (2026-10-31), dbmate 2.38.1 (2026-11-09), `github.com/ogen-go/ogen` 1.24.0, `github.com/swaggo/swag` 1.16.6, `github.com/golang-migrate/migrate/v4` 4.20.1, `gorm.io/gorm` 1.31.2, `github.com/go-chi/chi/v5` 5.5.0, `github.com/labstack/echo/v4` 4.16.3, `github.com/gin-gonic/gin` 1.12.0 and `go.opentelemetry.io/contrib/bridges/otelslog` 0.21.0.

goose 3.30.0 came out on the day of the check. Nothing in the prototype depends on a change in it, so 3.29 would do if the template has to avoid a release that new.

## What the prototype is

```text
backend/
  main.go                  the server: logger, telemetry, pool, routes, graceful shutdown
  internal/api/api.go      the routes, registered through huma
  internal/api/api_test.go /healthz and /readyz tests with httptest
  internal/telemetry/      the OpenTelemetry setup and the slog handler that adds trace IDs
  cmd/openapi/main.go      writes the OpenAPI document to stdout
  cmd/migrate/main.go      "migrate up" and "migrate new <name>", built on goose
  migrations/              goose SQL files
  openapi.json
  .golangci.yml
  mise.toml
```

The Go code is 342 lines. The final `go.mod` requires 10 modules directly (huma, pgx, goose and seven OpenTelemetry modules) and 26 indirectly. `go.sum` has 116 lines.

The mise tasks:

| Task | Command |
|---|---|
| `dev` | `go run .` |
| `test` | `go test ./...` |
| `lint` | `golangci-lint run` |
| `build` | `go build -o bin/ . ./cmd/migrate` |
| `openapi` | `go run ./cmd/openapi > openapi.json` |
| `migrate` | `go run ./cmd/migrate up` |
| `migrate-new` | `go run ./cmd/migrate new` (mise appends the name) |

A script ran every check below in one go and printed 23 `PASS` lines. It removed its containers on exit. The prototype's README lists the command behind each check, copied here because the prototype isn't kept. Commands run in `backend/`, `frontend/` holds only orval and TypeScript, and `otel/config.yaml` configures the collector's OTLP/HTTP receiver and file exporter.

| Point | Command | Passes when |
|---|---|---|
| Tasks | `mise run lint`, `mise run test`, `mise run build` | each exits 0, and `bin/backend` and `bin/migrate` exist |
| Type errors fail lint | append `var _ int = "x"` to `internal/api/api.go`, then to `internal/api/api_test.go`, and run `mise run lint` | each exits 1 |
| `openapi.json` export | `mise run openapi` | `openapi.json` is unchanged |
| Served document matches | start `bin/backend`, then `curl -s localhost:3000/openapi.json \| jq -S . \| diff - <(jq -S . openapi.json)` | no diff |
| Health without env | start `bin/backend` with `DATABASE_URL` and `OTEL_EXPORTER_OTLP_ENDPOINT` unset, then `curl localhost:3000/healthz` | 200 |
| Postgres | `docker run -d --name proto104-pg -e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=demo -p 55432:5432 postgres:18`, `export DATABASE_URL=postgresql://app:app@localhost:55432/demo` | |
| No table at startup | start `bin/backend`, then `docker exec proto104-pg psql -U app -d demo -tAc "select count(*) from pg_tables where schemaname='public'"` | prints 0 |
| `/readyz` | `curl localhost:3000/readyz` with Postgres up, then after `docker stop proto104-pg` | 200, then 503, while `/healthz` stays 200 |
| `migrate` | `mise run migrate` twice, with no `.env` | first prints `applied ...create_example.sql`, second prints `no pending migrations`, both exit 0 |
| `migrate-new` | `mise run migrate-new add_example` | `migrations/<timestamp>_add_example.sql` exists |
| OTLP receiver | `docker run -d --name proto104-otel -p 54318:4318 -v $PWD/../otel/config.yaml:/etc/otelcol-contrib/config.yaml -v $PWD/../otel/out:/out otel/opentelemetry-collector-contrib:0.165.0` | |
| Traces, metrics, logs | `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:54318 bin/backend`, request `/healthz`, `/readyz` and `/openapi.json`, stop the server, read `../otel/out/telemetry.jsonl` | one span `GET /openapi.json` with `service.name` `demo-backend`, no health spans, `http.server.request.duration` with `http.response.status_code`, and the request's log line has the span's `trace_id` |
| orval | in `frontend/`: `pnpm exec orval && pnpm exec tsc -p .` | exits 0, and `src/client.gen.ts` has `useGetHealthz` |
| Static image | `CGO_ENABLED=0 GOOS=linux go build -o ../linux/backend .`, then `docker run --rm -p 3000:3000 -v $PWD/../linux:/app:ro gcr.io/distroless/static-debian13:nonroot /app/backend` | `/healthz` returns 200 |

The results:

- `mise run lint`, `test` and `build` pass. `build` writes `bin/backend` and `bin/migrate`.
- A type error in `internal/api/api.go` makes `lint` exit 1, and so does one in `internal/api/api_test.go`. golangci-lint type-checks test files by default.
- `mise run openapi` writes the same bytes as the committed `openapi.json`, on every run.
- `GET /openapi.json` on the running server equals the exported file after sorting keys with `jq -S`.
- With `DATABASE_URL` and `OTEL_EXPORTER_OTLP_ENDPOINT` unset, the server starts and `/healthz` returns 200.
- Against a `postgres:18` container, `/readyz` returns 200. After `docker stop`, `/readyz` returns 503 and `/healthz` returns 200.
- Starting the server against an empty database creates no table.
- `mise run migrate` with only `DATABASE_URL` set and no `.env` applies the initial migration and exits 0. A second run prints `no pending migrations` and exits 0.
- `mise run migrate-new add_example` creates `migrations/<timestamp>_add_example.sql`.
- With `OTEL_EXPORTER_OTLP_ENDPOINT` pointing at the collector, a request to `/openapi.json` produces a span named `GET /openapi.json` with `service.name` `demo-backend`. `/healthz` and `/readyz` produce no spans.
- The collector receives `http.server.request.duration` with `http.response.status_code` and `http.route` attributes.
- The request's log line is JSON with the same `trace_id` as its span. The startup line has no `trace_id`.
- orval 8.47.0 generates a client from the exported `openapi.json`. `tsc` 7.0.2 accepts it, and it has `useGetHealthz` and `useGetReadyz`, the same hook names the `backend: ts` document gives.
- `CGO_ENABLED=0 GOOS=linux go build` gives a static binary that serves `/healthz` on `distroless/static-debian13:nonroot`. The prototype ran the binary in that image with a volume mount, without a Dockerfile.

## The HTTP framework and the OpenAPI document

The requirement is the one `backend: ts` meets with `@hono/zod-openapi`: one definition per route produces both the served route and its entry in `openapi.json`, so the two can't drift, and a task can export the document without starting the server.

### The options

| | huma on `net/http` | oapi-codegen | swaggo/swag | ogen |
|---|---|---|---|---|
| Source of truth | Go code: `huma.Register` with typed input and output structs | a hand-written OpenAPI file | comments above each handler | a hand-written OpenAPI file |
| Can the served routes and the document drift? | Not for routes registered through huma. A route added straight to the `ServeMux` is served and missing from the document | Not if the generated code is regenerated. A freshness check on the generated file catches a stale one | Yes. Nothing checks the comments against the code | Same as oapi-codegen |
| OpenAPI version | 3.1.0 | 3.0 and 3.1 | Swagger 2.0 in 1.x | not checked |
| Modules added to `go.mod` | 2 (huma and `google/uuid`) | 17 when run through `go tool`, none at runtime for these two routes | not checked | not checked |
| Generated Go code | none | 437 lines for two routes | not checked | not checked |
| What a model writes to add a route | one `huma.Register` call and two structs, copied from the next route | a block of OpenAPI JSON by hand, then a handler method that matches the generated interface | a handler and a comment block in swag's own syntax | the same as oapi-codegen |
| Prototyped | yes, end to end | yes, generate, build and serve | no | no |

huma's `humago` adapter runs on the standard library's `http.ServeMux`, which has handled method and path-parameter patterns since Go 1.22. The handlers stay `net/http` handlers, and the middleware is plain `func(http.Handler) http.Handler`. huma also ships adapters for chi, echo, gin and others, so a later move to one of those routers changes one line.

oapi-codegen 2.8.2 also passed. It generated a `std-http` strict server from the very `openapi.json` huma had exported, including the 3.1 `type: ["array", "null"]` form, and the server answered `/healthz`. It's spec-first, so drift is prevented by regenerating and compiling. It lost on two points. A model has to write OpenAPI JSON by hand, which is verbose and easy to get subtly wrong, and every new route regenerates a 400-line file the author can't review. Adding it with `go tool` puts the generator and kin-openapi in the module graph.

swaggo was rejected without a prototype. Its comments aren't checked against the code, so the document can drift, which `backend: ts` rules out. ogen wasn't prototyped either. It's spec-first like oapi-codegen, so a model would still write OpenAPI by hand.

The router alternatives (chi, echo, gin) add nothing the standard `ServeMux` lacks for this template. echo and gin also bring their own context types, which gives weaker models a second way to write a handler.

How each option scores on the author's constraints:

| | Close to the standard library | Small | Easy for weaker models | Drift prevented |
|---|---|---|---|---|
| huma on `ServeMux` | yes, `net/http` handlers and middleware | 2 modules, no generated code | copy one `Register` block | for every route registered through huma |
| oapi-codegen | yes, generates `net/http` code | no runtime modules, 437 generated lines | must write OpenAPI by hand | yes, with a freshness check on generated code |
| swaggo | yes | not checked | must learn swag's comment syntax | no |
| ogen | not checked | not checked | must write OpenAPI by hand | yes, with a freshness check on generated code |

### What #105 needs to know about huma

- `huma.DefaultConfig` turns on three things the template doesn't want. It adds a `$schema` field to every JSON response body and a `Link` header, serves a docs page at `/docs`, and serves `/schemas/...`. The prototype turns all three off:

  ```go
  config := huma.DefaultConfig("backend", "0.0.0")
  config.DocsPath = ""
  config.SchemasPath = ""
  config.CreateHooks = nil // removes the $schema field
  ```

  With that, `/healthz` returns `{"status":"ok"}`, the same body as `backend: ts`.
- Set `OperationID` to `getHealthz` and `getReadyz`. orval names the hooks from it, and those names match what it derives from the `backend: ts` document, so the frontend works unchanged.
- A route registered like this is served and documented by the same call:

  ```go
  huma.Register(api, huma.Operation{
      OperationID: "getReadyz",
      Method:      http.MethodGet,
      Path:        "/readyz",
      Summary:     "Readiness: the service can take requests.",
      Errors:      []int{http.StatusServiceUnavailable},
  }, func(ctx context.Context, _ *struct{}) (*StatusOutput, error) {
      if err := ready(ctx); err != nil {
          return nil, huma.Error503ServiceUnavailable("not ready")
      }
      return &StatusOutput{Body: Status{Status: "ok"}}, nil
  })
  ```

  `Status` is `struct{ Status string `json:"status" enum:"ok"` }`, and huma turns the `enum` tag into the schema's `enum`.
- huma adds error responses on its own. Every operation gets a `default` response, and an operation with `Errors` also gets 500. They use an `ErrorModel` schema with the content type `application/problem+json` (RFC 9457). A 503 body is `{"title":"Service Unavailable","status":503,"detail":"not ready"}`. The document is therefore longer than the `backend: ts` one, and orval generates `ErrorModel` and per-status response types from it. `tsc` accepts them.
- The export needs no server. `cmd/openapi` builds the API on a fresh `ServeMux` and marshals `api.OpenAPI()` with `json.MarshalIndent`. huma writes object keys in sorted order, so the output is the same bytes on every run. The served `/openapi.json` is compact, so compare it after `jq -S`.
- huma's own `go.mod` lists gin, echo, fiber and others for its adapters. Go's module graph pruning keeps them out of the backend's `go.mod`. Only `google/uuid` arrives with huma.
- The drift gap: `mux.HandleFunc` on the same `ServeMux` serves a route the document doesn't list. `backend: ts` has the same gap with `app.get`. The generated `AGENTS.md` should say every route goes through `huma.Register`.
- huma has one main maintainer, who wrote more than 900 of its commits. It's MIT-licensed and had eight releases between February and October 2026. If it stops, the handlers are plain Go functions and the routes are on a standard `ServeMux`, so moving to oapi-codegen means writing the spec from the last exported `openapi.json`, which oapi-codegen already accepted.

## The database library

| | pgx v5 with sqlc | pgx v5 alone | `database/sql` with pgx's driver | GORM |
|---|---|---|---|---|
| How a query is written | SQL in `queries.sql` with a `-- name:` comment, then `sqlc generate` writes a typed Go function | SQL string, then `Scan` into variables in the same order as the columns | the same as pgx alone, through the standard interface | Go method chains and struct tags |
| When a wrong column name fails | at `sqlc generate` | at runtime | at runtime | at runtime |
| Modules added | pgx only, since the generated code imports only pgx | pgx | pgx | GORM, its Postgres driver, and pgx under it |
| Extra tool | the sqlc binary, pinned in mise | none | none | none |
| Prototyped | yes | yes, for `/readyz` | yes, inside `cmd/migrate` | no |

The choice is pgx v5 with sqlc. pgx is the Postgres driver every other option here uses underneath. sqlc is the Go counterpart of Prisma's generated client. It reads the goose migrations as the schema and writes typed functions from SQL, so a model writes SQL it can test in `psql`, and a typo fails before the code compiles. The generated code adds no module beyond pgx.

GORM was rejected without a prototype. Its `AutoMigrate` invites migrating at startup, which #107 forbids, and its query builder hides the SQL from the author.

What the prototype showed:

- `pgxpool.New` doesn't connect until the first query, so the server starts while Postgres is down. `/readyz` runs `SELECT 1` with a 2-second timeout.
- pgx accepts the `.env.example` form `postgresql://app:app@localhost:5432/demo` with no `sslmode`.
- sqlc 1.31.1, configured with `schema: migrations` and `sql_package: pgx/v5`, read the goose migration, skipped its `-- +goose Down` section, and generated `CreateExample` and `ListExamples`. A test ran both against Postgres and passed.
- `sqlc generate` failed with `column "nmae" does not exist` on a misspelt column.
- `sqlc generate` fails when the queries file has no query. The template therefore needs a starter table and one query, or it ships sqlc's config only once the first query exists. #107 decides which, and adds a freshness check for the generated code like the one for `openapi.json`.

## The migration tool

| | goose as a library in `cmd/migrate` | goose CLI through `go tool` | dbmate | golang-migrate |
|---|---|---|---|---|
| `migrate` | `go run ./cmd/migrate up` | `go tool goose ... up` | `dbmate up` | `migrate -path ... -database ... up` |
| `migrate-new add_example` | writes `migrations/<timestamp>_add_example.sql` | the same | writes `db/migrations/<timestamp>_add_example.sql` | writes separate up and down files |
| Reads `DATABASE_URL` | the 60-line command reads it | needs it passed as an argument | yes, natively | needs it passed as an argument |
| Effect on `go.mod` | goose plus 3 small modules | 51 more modules, including drivers for ClickHouse, MySQL, SQL Server, SQLite and YDB, and newer versions of shared modules such as gRPC | none | none if installed as a binary |
| In the production image | `bin/migrate`, built from the same module | needs the Go toolchain | a second, 31.6 MB binary to download | a second binary to download |
| Prototyped | yes | added and measured, then removed | yes, `new` and `up` twice | no |

The choice is goose as a library, in a small `cmd/migrate` command. The deploy step that runs migrations (roadmap, `database`) can use `bin/migrate` from the backend's own image, built from the same `go.mod`, so the author's Dockerfile builds two binaries and downloads no tool. Renovate's gomod manager bumps goose with the other modules.

dbmate 2.38.1 also worked. It reads `DATABASE_URL` without any wrapper, and `up` twice exited 0 both times. It would add a separate binary to mise and to the image. It also writes `db/schema.sql` after every `up` unless told not to. golang-migrate was rejected without a prototype. Each migration is two files, and a failed migration leaves the database marked dirty until someone runs `force`.

The `migrate` command's core:

```go
db, err := sql.Open("pgx", url) // the driver from github.com/jackc/pgx/v5/stdlib
provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"))
results, err := provider.Up(context.Background())
```

`migrate new <name>` calls `goose.Create(nil, "migrations", name, "sql")`, which writes a file with `-- +goose Up` and `-- +goose Down` sections and logs `Created new file: ...` through the standard `log` package.

## The logger

The choice is the standard library's `log/slog` with its JSON handler, wrapped in a 30-line handler that adds `trace_id` and `span_id` when the context holds a span:

```go
func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
    if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
        r.AddAttrs(
            slog.String("trace_id", sc.TraceID().String()),
            slog.String("span_id", sc.SpanID().String()),
        )
    }
    return h.Handler.Handle(ctx, r)
}
```

slog carries trace IDs cleanly, so the default stands.

- The wrapper must also override `WithAttrs` and `WithGroup` and wrap their result. Without that, a logger made with `logger.With(...)` returns the inner JSON handler and loses the trace IDs.
- The trace ID only reaches a line written with a context, such as `slog.InfoContext(ctx, ...)`. golangci-lint's `sloglint` with `context: all` fails `lint` on `slog.Info` and the other calls without one. The prototype's lint caught `slog.Error` in `main.go` this way.
- The request log middleware has to run inside the `otelhttp` handler, or its context holds no span yet.
- The `otelslog` bridge 0.21.0 was rejected. It sends log records to an OTLP log exporter, while #106 asks for JSON lines on stdout. zap and zerolog would add a module for something slog already does.

## The OpenTelemetry setup

- `telemetry.Start` does nothing when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, so the server runs with the default no-op providers. When it's set, it creates `otlptracehttp` and `otlpmetrichttp` exporters, which read the variable themselves, plus a batch span processor and a periodic metric reader. It sets the resource's `service.name` in code and lets `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` override it (`resource.WithFromEnv()` after `resource.WithAttributes`). It returns a shutdown function that flushes both providers, and `main` calls it on exit.
- `otelhttp.NewHandler` wraps the whole `ServeMux`. Its `WithFilter` option drops `/healthz` and `/readyz` from both spans and metrics.
- otelhttp 0.72.0 names the span from the `ServeMux` pattern after the handler runs, so the span is `GET /openapi.json`. It records `http.server.request.duration` with `http.route` and `http.response.status_code`. The span itself carries no `http.route` attribute.
- The OTLP HTTP exporters still bring `google.golang.org/grpc` into the module graph as an indirect dependency.
- The prototype set the metric export interval to 5 seconds so the check finishes quickly. The SDK default is 60 seconds. #106 decides the template's value.
- DB spans through `github.com/exaring/otelpgx` 0.13.0 are possible and weren't tried. They are left to #106 or #107.

## Lint

The prototype's `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - sloglint
  settings:
    sloglint:
      context: all
formatters:
  enable:
    - gofumpt
```

golangci-lint's standard linters found one real problem, an unchecked `db.Close()` error in `cmd/migrate`.

## Not proven here

- The author's Dockerfile and CI jobs. The prototype only ran a static binary in the distroless image.
- A `dev` task that restarts on change. `go run .` doesn't watch files. `node --watch` does this for `backend: ts`. #105 chooses between plain `go run .` and a watcher such as `mise watch`.
- Rendering through `sow new`, the health tests, and the `.env` loading that mise does in a generated project.
- Routes with path parameters, query parameters or request bodies, through huma and through orval.
- Running on Linux. Every check ran on macOS on Apple Silicon, with Docker 29.7.2.
- DB spans through `otelpgx`.
