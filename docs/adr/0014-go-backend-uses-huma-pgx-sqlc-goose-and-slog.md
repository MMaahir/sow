# The Go backend uses huma, pgx with sqlc, goose and slog

`backend: go` has to meet the stack interface (ADR 0004) the way `backend: ts` does, and the author doesn't read Go, so its code has to stay small, close to the standard library, and easy for weaker coding models to extend by copying what's there. A prototype on 2026-11-10 proved this set together on Go 1.27.4. `docs/research/backend-go-prototype.md` has the comparison, the versions and the evidence.

- **HTTP and OpenAPI.** The standard library's `net/http` server and `http.ServeMux`, with huma v2 (`github.com/danielgtaylor/huma/v2`) through its `humago` adapter. Each route is one `huma.Register` call with typed input and output structs, and that call both serves the route and adds it to the OpenAPI 3.1 document. A small `cmd/openapi` command writes the document to `openapi.json` without starting a server, so the served routes and the exported file come from one definition, as they do with `@hono/zod-openapi`. orval generates the frontend's client from it.
- **Database.** pgx v5 (`github.com/jackc/pgx/v5`), with one `pgxpool` pool created at startup. Queries are written in SQL. sqlc reads the migrations as the schema, checks each query against it, and generates a typed Go function per query. sqlc is a binary pinned in mise, and its generated code imports only pgx.
- **Migrations.** goose v3 (`github.com/pressly/goose/v3`) as a library, inside a small `cmd/migrate` command in the backend's own module. `mise run migrate` runs `migrate up` and `mise run migrate-new <name>` runs `migrate new <name>`. The build produces `bin/migrate` beside `bin/backend`, so the migration step can run from the backend's own image. The backend never migrates at startup.
- **Logger.** The standard library's `log/slog` with its JSON handler, wrapped in a handler that adds `trace_id` and `span_id` from the context. golangci-lint's `sloglint` with `context: all` makes every log call pass a context.
- **OpenTelemetry.** The Go SDK with the OTLP/HTTP trace and metric exporters, set up only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, and `otelhttp` wrapping the `ServeMux`, with a filter that drops `/healthz` and `/readyz`.

## Considered options

- **oapi-codegen, spec-first.** It passed in the prototype and prevents drift by generating the server from the document. It was rejected because a model would write OpenAPI JSON by hand for every route, and every change regenerates hundreds of lines of Go that nobody reviews by eye. It's the fallback if huma stops being maintained, because it accepts the document huma exports.
- **swaggo/swag.** Rejected. Its comments aren't checked against the code, so the document can drift.
- **chi, echo or gin as the router.** Rejected. Since Go 1.22, `http.ServeMux` matches methods and path parameters, which is all the template needs. echo and gin would add a second handler style. huma has adapters for all three if that changes.
- **pgx without sqlc.** It needs no extra tool, but a wrong column name or a `Scan` in the wrong order only fails at runtime. sqlc catches both when it generates.
- **GORM.** Rejected. It hides the SQL, and its `AutoMigrate` invites migrating at startup.
- **The goose CLI as a `go tool`.** Rejected. It pulls 51 extra modules, including drivers for five other databases, into the backend's `go.mod`.
- **dbmate.** It worked in the prototype and reads `DATABASE_URL` on its own. It was rejected because the image would need a second binary downloaded beside the backend, while `cmd/migrate` is built from the same `go.mod` and bumped by the same Renovate PRs.
- **golang-migrate.** Rejected. Each migration is two files, and a failed migration leaves the database marked dirty until someone runs `force`.
- **The `otelslog` bridge, zap or zerolog.** Rejected. The bridge sends logs to an OTLP exporter rather than stdout, and slog already writes JSON.

## Consequences

- The backend's `go.mod` requires huma, pgx, goose and seven OpenTelemetry modules directly. Go modules pin exact versions, so no range rule applies (ADR 0007).
- Every route must go through `huma.Register`. A route added straight to the `ServeMux` is served and missing from `openapi.json`, so the generated `AGENTS.md` says this, and the freshness check can't catch it.
- huma's default config needs three settings turned off: the `$schema` field it adds to response bodies, the `/docs` page and the `/schemas` routes. Operation IDs are set to names like `getHealthz`, so orval's hook names match the ones the `backend: ts` document gives.
- huma documents error responses as `application/problem+json` with an `ErrorModel` schema, so a Go backend's `openapi.json` is longer than a TypeScript one and its error bodies differ. The frontend's client works with both.
- huma has one main maintainer. If it stops, the handlers are plain Go functions on a standard `ServeMux`, and oapi-codegen can take over from the last exported `openapi.json`.
- sqlc fails when it has no query to generate, so the database starter needs a table and a query, or it adds sqlc only when the first query arrives. #107 decides which. sqlc's generated code gets a freshness check like `openapi.json`'s.
- The final image is a static binary on a distroless base. The prototype ran a `CGO_ENABLED=0` build on `gcr.io/distroless/static-debian13:nonroot`. The Dockerfile and CI jobs stay the author's.
