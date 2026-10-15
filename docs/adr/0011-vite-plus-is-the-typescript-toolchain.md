# Vite+ is the TypeScript toolchain

The TypeScript stacks (`frontend: react` and `backend: ts`) use Vite+ wherever it fits. It's one `vite-plus` devDependency with a `vp` CLI that bundles Vite, Vitest, Oxlint, Oxfmt and tsdown, configured through `vite.config.ts`, and it replaces installing and configuring each of those tools separately. sow itself and `backend: python` don't use it. Vite+ reached 1.0 on 2026-09-28.

Vite+ also offers a Node version manager, a task runner, git hooks and agent setup, and sow already gives each of those jobs to another tool. Ownership is split like this:

- **Vite+** installs Node and pnpm through the global `vp` CLI, which must be installed in managed mode. It reads `.node-version` (major version only, `24`) and the `packageManager` field, so local machines, `setup-vp` in CI and the `ghcr.io/voidzero-dev/vite-plus` build image all choose Node and pnpm from the same files. Mise never manages Node in a generated project, and the user's global mise config must not manage it either: mise tasks put mise's tools first on PATH, which would hide Vite+'s shims.
- **Mise** keeps Python, uv, `.env` loading and the tasks. The tasks stay the stack interface (ADR 0004), so `vp run` isn't used.
- **pre-commit** keeps the git hooks, because Python projects need it and two hook managers can't share `.git/hooks`. A local hook runs `vp check --fix` on staged TypeScript files. The template sets up no Vite+ hooks and has no `staged` block.
- **sow** keeps `AGENTS.md` and `.claude/settings.json`. The template never runs `vp config` or Vite+'s agent setup.

Each package's mise tasks run in that package's directory:

| Mise task | React | Hono |
|---|---|---|
| `dev` | `vp dev` | `node --watch src/index.ts` |
| `test` | `vp test` | `vp test` |
| `lint` | `vp check` | `vp check` |
| `build` | `vp build` | `vp pack` |

`vp dev` and `vp build` always run Vite and can't be redefined, so Hono uses Node 24's type stripping for development and `vp pack` (tsdown) for builds. tsdown leaves dependencies outside the bundle by default, and the backend image installs production dependencies in their own stage. `vp check` runs with `lint.options.typeAware` and `typeCheck` on, so it formats, lints and type-checks through tsgolint on TypeScript 7, and there's no separate `tsc` step. `typescript` stays a devDependency so editors get the language server.

Any TypeScript component brings a root pnpm workspace: a root `package.json`, `pnpm-workspace.yaml`, one root `vite-plus` and a root `vite.config.ts` for shared lint and format settings. Each package has its own `vite.config.ts` for build and test settings. `web-python` and `web-ts` then have the same layout.

## Pinning: an exception to ADR 0007

Vite+ needs two overrides in `pnpm-workspace.yaml`. `vite` is aliased to `@voidzero-dev/vite-plus-core` at the installed `vite-plus` version, and `vitest` is pinned to the Vitest version that `vite-plus` bundles, so plugins and dependencies can't pull in a second Vite or Vitest. Any `@vitest/*` package the template installs directly, such as `@vitest/coverage-v8`, must match that Vitest version too. These versions only work as a set, so `vite-plus` itself is also pinned exactly in `package.json`. With a `>=` range, `sow new` would install a newer `vite-plus` than the overrides name.

Renovate can't work out the bundled versions; a group only puts updates in one PR. Upgrades therefore work like this:

- Renovate bumps only `vite-plus` and the build image tag, in one weekly group with `rangeStrategy: pin`. It's turned off for `@voidzero-dev/vite-plus-core`, `vitest` and `@vitest/*`.
- A CI check fails if the overrides and `@vitest/*` versions don't match what `vp toolchain` reports.
- On the Renovate branch, `mise run upgrade-vite-plus` runs `pnpm exec vp migrate --no-interactive`, which re-pins the alias, `vitest` and `@vitest/*` to the new release's versions. Pushing that makes the check pass.

sow keeps the template's pins current the same way. Renovate's npm manager reads the template's `package.json` and `pnpm-workspace.yaml` files, which are kept free of Jinja so it can parse them. A regex manager is added only if that doesn't work. The generated-project health tests run the pin check on every preset.

## Consequences

- Every developer machine needs the global `vp` in managed mode. From 0.3, `sow new`, `sow update` and `sow add` check that `vp` is on PATH, that `vp env doctor` passes, and that `node` and `pnpm` inside the project's mise environment resolve to Vite+'s shims. The check runs before Copier when the components are already known: from the answers file, the `add` argument, or a preset or flags. Interactive `sow new` only learns the components from Copier's prompts, so there it runs before the follow-up step, when only a new directory exists. In 0.9, `sow doctor` also reports the global `vp` version.
- The CLI's follow-up step is `vp install`, which installs the exact pnpm named in `packageManager` (ADR 0010).
- These exact pins sit on lines the template owns, which ADR 0007 avoids for npm packages. A `sow update` conflicts when the project and the template have both moved Vite+ to different versions. That's the same cost ADR 0007 accepts for mise, Docker and Actions lines, but Vite+ releases more often, so it will happen more often. Conflicts are fixed by hand, and an update test covers this case.
- Every Vite+ upgrade takes one manual command on the Renovate branch. A workflow could push it automatically, but Renovate stops rebasing branches someone else has changed, and the workflow would need guarding against triggering itself.
- CI's TypeScript jobs run `setup-vp`, pinned to an exact release tag because its `v1` tag stopped getting updates at v1.15.0. Next, `mise-action` installs mise only to run the tasks. The pre-commit job runs with `SKIP=vp-check`, because the stack lint jobs already run a full `vp check`, so that job doesn't need Node.
- Build stages use `ghcr.io/voidzero-dev/vite-plus`, pinned to an exact tag. The frontend runs on nginx and the backend on `node:24-slim`, which declares the Node major a second time; Renovate bumps it.
- Starter code that passes doesn't prove type-checking is on. The generated-project health tests add a deliberate type error to frontend and backend code, test files included, and check that `lint` fails.
- Running `node` directly means the backend's tsconfig needs `erasableSyntaxOnly`, `verbatimModuleSyntax` and `rewriteRelativeImportExtensions`, relative imports need `.ts` extensions, and path aliases can't be used. A prototype at the start of 0.7 has to show the final container serving `/healthz` and `/readyz`, querying Postgres through Prisma, running migrations, exporting `openapi.json` and sending telemetry. It also records when `prisma generate` runs and checks that `vp install` works without mise or `.env`.
- The generated `AGENTS.md` sends agents to the mise tasks. They call `vp` directly only for commands without a task, because calling `vp` directly skips the `.env` loading mise does. They never call `vite`, `vitest`, `oxlint` or `oxfmt` directly. More detailed Vite+ guidance belongs in the skills repo's `typescript` plugin.
- Projects with a TypeScript component ship `.vscode/extensions.json` recommending `oxc.oxc-vscode` and `.vscode/settings.json` making it the formatter, so editors format with the root Vite+ config.
- The CI audit job keeps `pnpm audit`; `vp pm audit` just passes the call through to it.
