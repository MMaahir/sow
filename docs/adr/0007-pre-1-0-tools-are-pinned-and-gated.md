# Pre-1.0 tools are allowed if pinned exactly and updated through CI

The toolchain includes pre-1.0 tools on purpose, such as oxfmt. Pre-1.0 tools can change behaviour in minor releases, so each one is pinned to an exact version. Renovate opens a PR for every update, and CI must pass before it merges, so breakage shows up as a failing PR and never on `main`. The same rule applies in sow and in every generated project. Alpha tools are still too early. Zensical replaced Material for MkDocs when Zensical reached beta with its 0.1.0 release on 2026-11-05.

In generated projects, the exact pin lives in the lockfile (`pnpm-lock.yaml`), not in the manifest. `package.json` uses lower bounds with no upper bound (`>=`), and Renovate's npm manager, which handles pnpm, runs with `rangeStrategy: in-range-only`. Every new version is inside a `>=` range, so Renovate still proposes every update, and it never edits the manifests. The template owns the manifests and raises a lower bound only when it needs a newer feature. If both the template and Renovate edited the same manifest lines, most `sow update` runs on an active project would conflict. Vite+ is the one exception: `vite-plus` and its overrides are pinned exactly in the manifests, because their versions only work as a set (ADR 0011).

Renovate's other managers (mise, Docker base images, GitHub Actions) still edit files the template also writes. Generated projects don't pin those by digest, so Renovate only opens PRs for new versions, which are rare. A conflict on one of those lines during `sow update` is expected and fixed by hand.

Amended 2026-11-09: the Python tools (ty, uv) left with the Python backend (ADR 0012). Go modules pin exact versions in `go.mod` by design, so `backend: go` and sow itself need no range rule.
