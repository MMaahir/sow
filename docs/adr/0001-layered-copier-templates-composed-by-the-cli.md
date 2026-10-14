---
status: superseded by ADR-0009
---

# Layered Copier templates, combined by our own CLI

Generated projects are built from several small Copier templates (layers), applied in order by the `sow` CLI. Each layer has its own answers file, so it can be updated separately. We rejected one big template full of `{% if %}` blocks because it gets hard to read and test as languages and options grow. We rejected writing our own generator because it would mean rebuilding Copier's templating and update logic. Combining layers is the CLI's job. Copier has no way to do it, and it's the main reason the CLI exists rather than being a thin wrapper around `copier copy`.

## Consequences

- Choosing and ordering layers, presets, and running `copier update` across all answers files all live in the CLI.
- Copier can't cleanly take a layer back out of a project, so there is no `sow remove`.
