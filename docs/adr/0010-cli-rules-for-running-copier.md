# Rules for how the CLI runs Copier

The template uses no Copier tasks, migrations or Jinja extensions. Rendering only writes files, so an update can re-render the old version reliably and nobody needs `--trust`.

`sow update` and `sow add` always target the CLI's own version (`--vcs-ref v<cli version>`) and pass `--defaults` so Copier doesn't ask every question again. A project is therefore never on a different template version from the CLI that last touched it, and `add` doesn't need a separate version check. Copier is pinned exactly, because an update re-renders the old template with the Copier installed now, and a Copier change could make that render differ.

After Copier finishes, the CLI works through these steps in order and stops at the first failure:

1. Exit non-zero if `<<<<<<< before updating` markers or `*.rej` files remain.
2. Run the follow-up steps. Each one runs only if its file exists: `uv lock` in `backend/` if `backend/pyproject.toml` exists, and `vp install` at the root if `pnpm-workspace.yaml` exists. Every project with a TypeScript component has a root pnpm workspace, and `vp install` installs the exact pnpm named in `packageManager` (ADR 0011). `minimal` has none. If a step fails, the CLI exits non-zero, prints the failed command and its directory, and leaves Copier's changes in place. You fix the cause and run that command yourself.
3. For `new` only: copy `.env.example` to `.env` if `.env` doesn't exist, run `git init` and make the first commit. `update` and `add` leave their changes uncommitted for review.

The template never renders `.env`. It's gitignored, so if Copier wrote it, git couldn't restore its old contents. Because of that, and because Copier refuses to start when the repo has uncommitted or untracked files, `git reset --hard && git clean -fd` safely undoes a failed update.

## Consequences

- Generated projects keep exact versions only in lockfiles (ADR 0007), so template updates and the project's own Renovate PRs rarely edit the same lines. The exception is the Vite+ pins in `package.json` and `pnpm-workspace.yaml` (ADR 0011), which conflict when the project and the template have moved Vite+ to different versions.
