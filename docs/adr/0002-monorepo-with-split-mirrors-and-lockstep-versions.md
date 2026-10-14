---
status: superseded by ADR-0009
---

# Monorepo with split mirrors, released in lockstep

Copier allows only one template per git repo, and `copier update` needs git tags on that repo. We still develop the CLI and every layer in this one repo, under `templates/<layer>/`. On each release, CI runs `git subtree split` to push each layer to a read-only split mirror and tags every mirror with the same version. A change that touches several layers lands in one PR, and one CI run tests every layer at the same commit, which the pairwise layer-combination tests rely on. The CLI at version X always uses layers at version X.

## Considered options

- **One repo per layer:** simpler, but a change across layers takes several PRs, and nothing guarantees the layers were tested together.
- **Templates bundled inside the Python package:** breaks `copier update`, because the templates no longer have git tags.
- **Independent version numbers per layer:** more precise, but the CLI would need a table of which layer versions work together.

## Consequences

- Never edit the split mirrors directly, and never move a tag after a release. Copier picks versions by comparing PEP 440 tags.
- Releases use Conventional Commits and release-please.
