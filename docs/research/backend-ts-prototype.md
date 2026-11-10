# backend: ts prototype

Run on 2026-11-09 for ticket #34, in a throwaway pnpm workspace outside the repo. It checks the setup ADR 0011 describes against the current releases. The global `vp` CLI wasn't installed on the machine, so every `vp` command ran as `pnpm exec vp` from the project's own `vite-plus`. The parts that need the global CLI are listed at the end.

## Versions proven together

| Package | Version |
|---|---|
| `vite-plus` | 1.5.1 (bundles Vite 8.3.7, Vitest 5.1.0, Oxlint 1.91.0, Oxfmt 0.76.0, tsdown 0.23.4) |
| `hono` | 4.13.17 |
| `@hono/zod-openapi` | 1.6.3 (needs `zod` 4 and `hono` 4.10 or later) |
| `@hono/node-server` | 2.1.6 |
| `zod` | 4.6.7 |
| `typescript` | 7.0.2 |
| pnpm | 12.4.2 |
| Node | 24.21.0 and 25.9.0 |

## What works as ADR 0011 says

- `node src/index.ts` and `node --watch src/index.ts` start the server through Node's type stripping, with `erasableSyntaxOnly`, `verbatimModuleSyntax` and `rewriteRelativeImportExtensions` in the tsconfig and `.ts` on relative imports.
- `vp pack` with `pack.entry` set in the package's `vite.config.ts` writes `dist/index.mjs` and leaves dependencies outside the bundle. `node dist/index.mjs` serves `/healthz` and `/readyz`.
- `vp check` with `lint.options.typeAware` and `typeCheck` in the root `vite.config.ts` formats, lints and type-checks. A type error in a source file fails it, and so does a type error in a test file.
- `vp check --fix <paths>` from the workspace root works on files inside a package, which is what the pre-commit hook does.
- `app.getOpenAPI31Document()` from `@hono/zod-openapi` returns the document without starting a server, so a script run with `node` writes `openapi.json`.
- Tests import from `vite-plus/test`. No direct `vitest` dependency is needed.

## What differs from ADR 0011

- **Pins live in a pnpm catalog.** `vp migrate` 1.5.1 moves `vite-plus` and the `vite` alias into the `catalog` block of `pnpm-workspace.yaml`. Each `package.json` then says `"vite-plus": "catalog:"`, and the override is `vite@*: "catalog:"`. ADR 0011 describes exact versions in `package.json` and a literal override. The template uses the catalog form, so `vp migrate` changes only version numbers on an upgrade.
- **No Vitest override without a direct dependency.** `vite-plus` depends on an exact Vitest, so `vp migrate` writes no `vitest@*` override when no package declares `vitest` or `@vitest/*` itself. The template declares neither, so it has no Vitest pin. The pin returns the day a package adds `@vitest/coverage-v8` or similar.
- **`vp migrate` also writes** `minimumReleaseAgeExclude` entries for the Vite+ packages and a `peerDependencyRules` block allowing any `vite`. The template keeps both.
- **`vp test` watches in a terminal.** The `test` task runs `vp test run`, which always runs once.
- **Lint scope depends on `.gitignore`.** Without `node_modules/` and `dist/` in `.gitignore`, `vp check` lints 973 files and fails. The generated `.gitignore` lists both, and the root config also ignores `dist`.
- **Oxfmt would reformat `openapi.json`**, and the freshness check compares it with a fresh export, so the root config leaves the file out of formatting.
- **New since the ADR:** `vpx <file>` runs TypeScript files and `vp toolchain --json` prints the bundled versions as JSON. The template keeps `node --watch`, as the ADR decided. The pin check in ticket #36 can read `vp toolchain --json`.
- **Vitest 5.1 supports Node `^22.18.0 || ^24.11.0 || >=26.0.0`.** `.node-version` stays at `24`.

## Not proven here

These need the global `vp` in managed mode and are checked by the health tests once it's installed:

- `vp install` as the follow-up step, on a machine where `vp` chooses Node and pnpm from `.node-version` and `packageManager`.
- `vp env doctor`, and that `node` and `pnpm` inside the project resolve to Vite+'s shims.
- The container image. The Dockerfile is the author's (#42).
