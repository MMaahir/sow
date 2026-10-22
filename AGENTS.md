# sow

A CLI that generates and updates Python and TypeScript web projects from one Copier template. It's a personal internal tool, built to learn SRE, DevOps and platform engineering.

Read `docs/roadmap.md` before planning or implementing anything: it holds the decided scope, stack and release plan. `docs/related-projects/` describes the agent skills and `platform` repos that sow depends on.

Read `docs/agents/copier-gotchas.md` before writing `copier.yml`, template files or CLI tests: it lists Copier, Jinja and test-harness behaviour the docs don't state.

## Agent skills

### Issue tracker

Specs and tickets are GitHub issues on `MMaahir/sow`, managed with the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five default triage labels, each named after its role. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root, created when the first term or decision is resolved. See `docs/agents/domain.md`.
