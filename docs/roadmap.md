# sow roadmap

Decided in planning sessions on 2026-10-13, 2026-10-14 and 2026-10-15, and reworked on 2026-11-09 when the CLI moved to Go. The reasoning behind the hard-to-reverse decisions is in `docs/adr/`. Terms are defined in `GLOSSARY.md`.

## Goal

A CLI that creates a project for a chosen language and wires up agent skills, CI/CD, hosting, infrastructure and observability. It's an internal tool that keeps evolving, and a way to learn SRE, DevOps and platform engineering to a production standard.

"Production grade" means each piece meets this standard when it ships: tested, versioned, documented and reversible. It doesn't mean every feature has to exist at once. When two designs both work, pick the simpler one.

## Who writes what

Agents write the application code and the author writes the infrastructure by hand ([ADR 0013](adr/0013-agents-write-code-the-author-writes-infrastructure.md)). The split applies to this repo and to the template.

| Owner | Files |
|---|---|
| Author | GitHub Actions workflows, branch protection, the release pipeline, Dockerfiles, Compose files, Helm charts, Argo CD manifests, Terraform, Renovate config, Prometheus rules and SLOs, the `platform` repo |
| Agents | All Go code, the template's application code (the Hono backend, the React frontend, their tests and configs), mise tasks, pre-commit config, `AGENTS.md`, docs |

Tickets for the author carry the `ready-for-human` label. Tickets for agents carry `ready-for-agent`. An agent never creates or edits a file on the author's side. Where an agent ticket needs one of those files, it leaves the gap and the tests skip the checks that depend on the missing file.

The author doesn't read Go. An agent PR merges once a second model has reviewed it, every finding is fixed or answered, and `mise run check` passes. `docs/agents/workflow.md` has the full procedure.

## Repo and releases

- One repo holds the CLI and the template. The template lives in `template/` and is compiled into the binary ([ADR 0012](adr/0012-go-cli-with-its-own-template-renderer.md)).
- **Versioning:** one version for the CLI and the template, taken from this repo's tags. Commits follow Conventional Commits, and release-please opens the release PR with `include-component-in-tag: false`, so tags are plain `vX.Y.Z`. Tags are never moved.
- **Release token:** release-please runs with a token from a personal GitHub App (`actions/create-github-app-token`), so checks run on the release PR. The App's private key is a repo secret; the tokens it mints last an hour.
- **Workflow:** trunk-based. `main` is protected, PRs are squash-merged, and PR titles use Conventional Commit format. The only required check is a final `ci-ok` job in the same workflow as every other job. It `needs` all of them, runs with `if: always()`, and fails if any of them failed or was cancelled. Jobs skipped on purpose count as passing. Without `if: always()`, GitHub skips `ci-ok` when a job fails, and a skipped required check counts as passing.
- **Distribution:** in 0.1, install with `go install github.com/MMaahir/sow/cmd/sow@v0.1.0`. From 0.2, GoReleaser publishes binaries with checksums and attestations to GitHub Releases and to a Homebrew tap.
- **Repos and licence:** public, MIT.

## CLI

Written in Go with cobra. Personal defaults (`user_name`, `user_email`, `github_user`) come from `~/.config/sow/config.yml`, or `$XDG_CONFIG_HOME/sow/config.yml` when that variable is set. Flags override them.

| Command | Job |
|---|---|
| `sow new <name>` | Applies a preset, or answers given as flags. Interactive, or `--no-input`. Runs the follow-up steps and makes the first commit. With `--github`, it also creates the repo, sets branch protection (requiring `ci-ok`), turns on secret scanning push protection and pushes |
| `sow update` | Updates the project to the CLI's version |
| `sow add <component> [<stack>]` | Updates the project to the CLI's version with the component turned on. `backend` and `frontend` need a stack, such as `sow add backend ts`. Exits without changes if the component is already on |
| `sow doctor` | Checks tool versions (including the global `vp` and `vp env doctor`), the skills marketplace install for Claude Code and Codex, and whether the project is behind the CLI's version |

The project name must be lowercase letters, digits and hyphens, start with a letter, end with a letter or digit, and be at most 53 characters. It becomes the Kubernetes namespace, the image names, the `<project>.localhost` URL and the Helm release name, and Helm limits release names to 53 characters.

`update` and `add` follow [ADR 0012](adr/0012-go-cli-with-its-own-template-renderer.md): they target the CLI's version, run the follow-up steps, fail if conflicts remain, and leave the changes uncommitted for review. No `remove` command is planned.

When a TypeScript component is on, `new`, `update` and `add` check that the global `vp` is installed and in charge of Node and pnpm ([ADR 0011](adr/0011-vite-plus-is-the-typescript-toolchain.md)). They exit with the install command if it isn't.

## Components and presets

Every generated project gets the always-on files and CI. Everything else is a component, turned on by an answer. The CLI validates prerequisites: `frontend`, `database`, `observability` and `deploy` each need a `backend`.

| Preset | Answers |
|---|---|
| `minimal` | No components |
| `web-ts` | `backend: ts`, `frontend: react`, `observability`, `database` |
| `web-go` | `backend: go`, `frontend: react`, `observability`, `database` |

A preset only sets answers at `sow new`. When a preset gains a component, existing projects get it only through `sow add`. `web-ts` arrives in 0.3 with the frontend, and before that `sow new --backend ts` turns the backend on. Backend and frontend stacks meet the stack interface ([ADR 0004](adr/0004-stack-interface.md)).

### What each part contains

- **Always on**
  - `AGENTS.md`, plus `.claude/settings.json` declaring the skills marketplace and the plugins for the chosen components ([ADR 0005](adr/0005-skills-via-plugin-marketplace.md))
  - mise for tool versions and tasks, alongside each tool's native version file (`.node-version`, `packageManager`, the `go` line in `go.mod`) so nothing depends on mise. Vite+, not mise, installs Node and pnpm ([ADR 0011](adr/0011-vite-plus-is-the-typescript-toolchain.md))
  - pre-commit, `.editorconfig`, `.gitignore` (including `.DS_Store`), an MIT licence and the Renovate config (`rangeStrategy: in-range-only` for pnpm, plus a weekly Vite+ group)
  - `.env.example`, and mise loading `.env`. The template never renders `.env`. `sow new` copies `.env.example` to `.env` only if `.env` doesn't exist
- **CI** (always on)
  - GitHub Actions workflows that call the mise tasks, ending in a `ci-ok` job
  - a check that PR titles follow Conventional Commits, pre-commit, and CodeQL on the workflows
  - each stack adds its own lint, test, audit (`pnpm audit` or `govulncheck`) and CodeQL jobs. TypeScript jobs start with `setup-vp`, pinned to an exact tag, then `mise-action`
  - the pre-commit job skips the `vp check` hook (`SKIP=vp-check`), because the TypeScript lint jobs already run it on the whole workspace
- **`backend: ts`**
  - Hono with `@hono/zod-openapi`, Node 24 LTS, pnpm
  - TypeScript 7, with Vite+ for tests, linting, formatting and type-checking (`vp test`, `vp check`)
  - `node --watch src/index.ts` for development, using Node's type stripping, and `vp pack` for builds, with dependencies left outside the bundle
  - a multi-stage Dockerfile: build on the Vite+ image, run on `node:24-slim`
- **`backend: go`**
  - the framework and libraries are decided when the 0.7 spec is written
  - a multi-stage Dockerfile on a distroless or scratch base image
- **`frontend: react`**
  - Vite+ (`vp dev`, `vp build`, `vp test`, `vp check`), React, TanStack Router and TanStack Query
  - an OpenAPI-generated client
  - Vitest with Testing Library, through `vp test`
  - a multi-stage Dockerfile: build on the Vite+ image, serve with nginx
- **Any TypeScript component**
  - a root pnpm workspace with one `vite-plus` and a root `vite.config.ts` for shared lint and format settings
  - `vite-plus`, its `vite` alias, `vitest` and `@vitest/*` pinned exactly, with a CI check that they match the bundled versions and a `mise run upgrade-vite-plus` task to re-pin them
  - `.vscode/extensions.json` and `.vscode/settings.json` setting up the Oxc extension as the formatter
- **`database`**
  - Postgres
  - Prisma (latest stable 7.x) for TypeScript. The Go library is decided with `backend: go`
  - migrations run as their own deploy step
- **`observability`**
  - the OpenTelemetry SDK in every service, and web-vitals on the frontend
  - structured JSON logs with trace IDs
  - a starting SLO per service, with generated Prometheus alert rules
  - `grafana/otel-lgtm` in Compose
- **`deploy`**
  - a Helm chart (`values-local.yaml`) and an Argo CD `Application`
  - multi-arch images (amd64 and arm64, built natively on matching runners), tagged with the git SHA
  - Trivy scans, SBOMs, and SLSA build provenance from `actions/attest-build-provenance`
  - Sealed Secrets
  - follows the platform contract ([ADR 0006](adr/0006-pull-based-gitops-against-a-separate-platform.md))

Generated projects use top-level `backend/` and `frontend/` directories, plus a root pnpm workspace when any component is TypeScript. In CI, the frontend's OpenAPI client is regenerated, and the check fails if it differs from the committed client. Playwright runs end-to-end smoke tests against the Compose stack.

Next.js is never used for server code: no API routes. If it comes back later, it does so as a second frontend stack.

## Toolchain rules

- Pre-1.0 tools are pinned exactly and updated through Renovate PRs that CI must pass. In generated projects the exact pin lives in the lockfile, and manifests use lower-bound ranges ([ADR 0007](adr/0007-pre-1-0-tools-are-pinned-and-gated.md)).
- sow itself is built with Go, pinned in `mise.toml` and `go.mod`. `gofumpt` formats it and `golangci-lint` lints it. `mise run check` runs the build, the linter and the tests, and it is the gate for every agent PR.
- sow keeps its Go dependencies few: cobra for commands, `huh` for prompts and a YAML library. Tests use the standard library only.
- TypeScript stacks use Vite+ wherever it fits. Vite+ installs Node and pnpm, mise runs the tasks, and pre-commit owns the hooks. Vite+ pins are the one exception to lockfile-only pins: Renovate bumps `vite-plus`, and `vp migrate` re-pins the rest ([ADR 0011](adr/0011-vite-plus-is-the-typescript-toolchain.md)).

## Testing sow

Tests are plain `go test`. They drive the CLI in-process through one function that takes arguments, input and output streams, an environment and a working directory, and returns an exit code. Nothing below that function is tested directly unless a ticket names the seam.

1. **CLI tests:** presets, answer validation, which follow-up steps run for which files, and the exit code when a follow-up step fails.
2. **Render tests:** files exist and contain what they should.
3. **Generated project health:** the generated project's own lint, tests and `docker build` pass. For TypeScript stacks, the Vite+ pin check passes, and a deliberate type error in source and test files makes `lint` fail. A check that depends on a file the author hasn't written yet, such as a Dockerfile, is skipped with a message until the file exists.
4. **Update tests:** generate with the previous release and update to the current one. Cases: an unedited project merges cleanly; a project with edits keeps them; an edit that clashes with a template change makes the CLI exit non-zero; an existing `.env` is left untouched; a project that upgraded Vite+ itself either merges cleanly or makes the CLI exit non-zero.

Levels 1 and 2 run in `mise run test`. Level 3 is slow and needs `vp` and Docker, so it runs in `mise run test-health`. Each PR tests every preset plus each backend stack on its own. A nightly job tests every valid combination of answers.

## AWS

Used only for sow's own docs site ([ADR 0008](adr/0008-aws-only-for-sows-own-infrastructure.md)). All of it is the author's work.

- **`infra/bootstrap`:** applied once from your laptop with local state. It creates the S3 state bucket (with native locking), the GitHub OIDC provider, a read-only plan role that trusts PR runs, an apply role limited to `main` and tags, and a $1 budget alert. The alert doesn't cap spending.
- **`infra/main`:** CI posts a plan on each PR (with `-lock=false`) and applies on merge. It manages the docs site (Material for MkDocs) on S3 behind CloudFront.
- **Teardown:** `infra/` documents how to `terraform destroy` both configs.

## Release plan

| Release | Agents | Author |
|---|---|---|
| 0.1 | The Go CLI with `sow new`, presets, answer flags and personal defaults. The always-on files. `backend: ts`. The `minimal` preset. Test levels 1 to 3 | sow's own CI and `main` protection. The generated CI. The backend Dockerfile. Renovate config. release-please with the GitHub App token. First real use: generate the agent skills repo |
| 0.2 | `sow update`, `sow new --github`, test level 4 with the first four update cases | GoReleaser, the Homebrew tap, checksums and attestations |
| 0.3 | `frontend: react` on Vite+, the OpenAPI client, Playwright tests, `sow add`, the `web-ts` preset, the CLI's `vp` check and the Vite+ update case | The frontend Dockerfile, the Compose file, the frontend CI jobs, the nightly combinations job |
| 0.4 | `observability` in the services: the OpenTelemetry SDK, JSON logs with trace IDs, web-vitals | `grafana/otel-lgtm` in Compose, the SLOs and the Prometheus alert rules |
| 0.5 | `database` for `backend: ts`: Prisma, the schema, the migration task | Postgres in Compose, the migration step in CI |
| 0.6 | Docs content | AWS: bootstrap, OIDC, the docs site |
| 0.7 | `backend: go`, the `web-go` preset | The Go Dockerfile and CI jobs |
| 0.8 | The CLI and template changes `deploy` needs | `deploy`: Helm, Argo CD, multi-arch builds, Trivy, SBOMs, build provenance. Needs `platform` running |
| 0.9 | `sow doctor` | |
| 1.0 | Shipped once two real projects use the template without major rework, and at least one of them has a backend, `database` and `observability`, and deploys to `platform` through Argo CD | |

## Open questions for 0.8

- **Image-tag write path.** CI can't push the new image tag to a protected `main`. Options: an image-update PR, a separate GitOps branch or repo, or a scoped bypass. Whichever is chosen must not trigger an endless build loop.
- **Platform contract changes.** Changing `platform` and the `deploy` component in the same release doesn't update projects already generated. Decide how existing projects learn about a contract change.

## Related projects

- `docs/related-projects/agent-skills.md`: the agent skills repo. sow's next real use.
- `docs/related-projects/platform.md`: the local cluster and shared services that the `deploy` component targets.
