# sow

A CLI for starting new projects without the setup work. One command creates a TypeScript or Go web project with CI, linting, testing, agent skills, observability and deployment already wired up, so a new idea starts at writing features instead of configuring tools. As the template improves, `sow update` brings those improvements into projects that already exist.

It's a personal tool with two jobs: making my own project start-ups fast and consistent, and giving me a real system to learn SRE, DevOps and platform engineering on. Coding agents write the CLI and the application code. I write the infrastructure by hand: the workflows, Dockerfiles, charts and Terraform. Every piece is held to a production standard: tested, versioned, documented and reversible.

## Status

The CLI is being rewritten in Go. Work on 0.1 has started. This repo also holds the design: the roadmap, the glossary and the decision records.

## What it will do

```sh
sow new my-app --preset web-ts   # create a project
sow add database                 # turn on a component later
sow update                       # pull in the latest template changes
sow doctor                       # check tools, skills and template version
```

A generated project comes with CI, pre-commit, Renovate, agent skills config and mise tasks already set up. Components add the rest:

| Component | Stacks |
|---|---|
| `backend` | `ts` (Hono), `go` |
| `frontend` | `react` (Vite+, TanStack Router and Query) |
| `database` | Postgres, with Prisma for TypeScript |
| `observability` | OpenTelemetry, structured logs, SLOs and alert rules |
| `deploy` | Helm, Argo CD and multi-arch images, deployed to a local `k3d` cluster |

Presets are named sets of answers: `minimal` (no components), `web-ts` and `web-go`.

Each project records the sow version it came from, so `sow update` can apply template changes with a three-way merge instead of you copying them over by hand.

## Release plan

| Release | Contents |
|---|---|
| 0.1 | The Go CLI, `sow new`, the TypeScript backend, CI, the `minimal` preset |
| 0.2 | `sow update`, GitHub repo setup, release binaries and a Homebrew tap |
| 0.3 | React frontend, `sow add`, the `web-ts` preset |
| 0.4 | `observability` |
| 0.5 | `database` |
| 0.6 | Docs site on AWS |
| 0.7 | Go backend, the `web-go` preset |
| 0.8 | `deploy` |
| 0.9 | `sow doctor` |
| 1.0 | Two real projects built with it, one deployed through Argo CD |

## Working on sow

```sh
mise install         # Go, the linters, pre-commit and Zensical
pre-commit install   # run mise run check before each commit
mise run check       # build, lint and test
mise run docs-build  # build the docs site into site/
```

## Docs

- [Roadmap](docs/roadmap.md): scope, stack, testing and the full release plan
- [Glossary](GLOSSARY.md): the terms used in this repo
- [Decision records](docs/adr/): why the hard-to-reverse choices were made
- [Related projects](docs/related-projects/): the agent skills repo and the `platform` cluster that sow depends on

## Licence

[MIT](LICENSE)
