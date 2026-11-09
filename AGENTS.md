# sow

A Go CLI that generates and updates TypeScript and Go web projects from one template. It's a personal internal tool, built to learn SRE, DevOps and platform engineering.

Agents write the code. The author writes the infrastructure files by hand and doesn't read Go.

## Before you write anything

1. Read `docs/agents/workflow.md`. It says how to pick a ticket, build it, get it reviewed and merge it.
2. Read `docs/agents/coding-guide.md` before writing Go or template files. Follow it exactly.
3. Read `docs/roadmap.md` before planning. It holds the decided scope, stack and release plan.

## Hard rules

- Never create or edit an infrastructure file: GitHub Actions workflows, Dockerfiles, Compose files, Helm charts, Argo CD manifests, Terraform, Renovate config, Prometheus rules. They belong to the author (ADR 0013). If your ticket seems to need one, leave the gap and say so in the PR.
- Never work on a ticket labelled `ready-for-human`.
- `mise run check` must pass before you commit.
- Use the terms in `GLOSSARY.md`. Don't use the words it lists under "Avoid".

## Agent skills

### Issue tracker

Specs and tickets are GitHub issues on `MMaahir/sow`, managed with the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five default triage labels, each named after its role. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

`docs/related-projects/` describes the agent skills and `platform` repos that sow depends on.
