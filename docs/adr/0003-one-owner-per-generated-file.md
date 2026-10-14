---
status: superseded by ADR-0009
---

# Each generated file has exactly one owning layer

Copier doesn't define what happens when two layers in one project render the same path. Each path therefore has exactly one file owner. Where a tool accepts separate files, each layer brings its own, such as `.github/workflows/<layer>.yml`. Files that must be a single file (`AGENTS.md`, `.pre-commit-config.yaml`, `.gitignore`) belong to `base`. The CLI passes `base` the list of active layers, and `base` renders the right content for each. Adding a layer means updating `base` again. The test suite fails if two layers would render the same path.
