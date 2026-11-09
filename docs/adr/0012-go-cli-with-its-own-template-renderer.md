# The CLI is written in Go and renders the template itself

The CLI moved from Python to Go on 2026-11-09, and the author dropped Python from the project. Copier is a Python library, so a Go CLI could only run it as a subprocess, which would keep uv and Python as runtime requirements of a Go binary. The CLI therefore renders the template itself with Go's `text/template` and performs updates with git's three-way file merge. sow uses a small part of Copier (one template, fewer than ten answers, no tasks or migrations), so the part to rebuild is small, and the update tests cover it end to end. This supersedes ADR 0009 and ADR 0010 and reverses ADR 0001's rejection of a home-made generator.

## The template format

The format is a compatibility contract. `sow update` renders the template of an older release with the current binary, so a change to these rules can change what an old release renders.

- The template lives in `template/` and is compiled into the binary with `go:embed`. A tag on this repo, such as `v0.1.0`, is both the CLI's version and the template's version.
- `template/base/` holds the always-on files. Every other directory belongs to one stack of one component and is named `<component>-<stack>/`, such as `backend-ts/` or `database-postgres/`. Every component answer is either `none` or a stack name, including components that have only one stack today. The CLI works out the directories from the answers by that naming rule alone, so the template needs no manifest.
- A component with no entry in the answers file is off. An answers file written by an older release has no entry for a component added later, and the old render must not pick that component up or take the current default.
- The CLI copies each selected directory onto the project root. The test suite fails if two directories that can be selected together contain the same path. A file that depends on several answers lives in `base/`.
- A file whose name ends in `.tmpl` is rendered with `text/template` and loses the suffix. Every other file is copied byte for byte. The delimiters are `[[` and `]]`, because GitHub Actions and Helm files already use `{{` and `}}`.
- Every answer is a string. Templates receive the answers and nothing else. They use no custom functions beyond the ones listed in `docs/agents/coding-guide.md`, and rendering never runs a command or reads the network.
- The answers file is `.sow-answers.yml` at the project root. It records the sow version and every answer.

## How update works

`sow update` and `sow add` always target the CLI's own version.

1. Refuse to start if the project has uncommitted or untracked files. `git reset --hard && git clean -fd` then safely undoes a failed update.
2. Read the answers file. Fetch this repo at the tag for the recorded version, and render its `template/` with the recorded answers. That is the old render.
3. Render the embedded template with the answers, changed by `add` if needed. That is the new render.
4. For each path, three-way merge with `git merge-file`: the project's file, the old render as the base, the new render. A file the project deleted stays deleted. A file only the new render has is added.
5. Exit non-zero if any file has conflict markers.
6. Run the follow-up steps. Each one runs only if its file exists, such as `vp install` at the root if `pnpm-workspace.yaml` exists. If a step fails, the CLI exits non-zero, prints the failed command and its directory, and leaves the changes in place.
7. For `new` only: copy `.env.example` to `.env` if `.env` doesn't exist, run `git init` and make the first commit. `update` and `add` leave their changes uncommitted for review.

The template never contains `.env`. It's gitignored, so git couldn't restore its old contents after a failed update.

## Considered options

- **A Go wrapper around Copier.** It keeps Copier's proven update logic and the existing Jinja template. It was rejected because the binary would need uv and Python at runtime, and the Go code would have to parse a subprocess's error output.
- **Running the old binary for the old render** (`go run ...@vOLD`). It guarantees the old render is exact, but every machine would need the Go toolchain. The frozen template format gives the same guarantee, and the update tests render real earlier tags to catch a break.

## Consequences

- Adding a component is an update with one answer changed. Switching an existing component to another stack isn't supported. No `remove` command is planned.
- `sow update` needs network access to this repo, unless the old version is already in the local cache.
- Tags must be plain `vX.Y.Z` and must never move.
- Generated projects keep exact versions only in lockfiles (ADR 0007), so template updates and the project's own Renovate PRs rarely edit the same lines. The exception is the Vite+ pins (ADR 0011).
