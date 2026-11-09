# Agents write the code and the author writes the infrastructure

sow exists to teach its author SRE, DevOps and platform engineering. The author doesn't know Go and doesn't want to spend the project's time on application code. From 2026-11-09, coding agents write all application code and the author writes every infrastructure file by hand. Having agents write everything was rejected because the infrastructure is the part the author wants to learn. Having the author write everything was rejected because it's too slow.

The line is drawn by file type, in this repo and in the template:

- **The author's:** GitHub Actions workflows, branch protection and repo settings, the release pipeline, Dockerfiles, Compose files, Helm charts, Argo CD manifests, Terraform, Renovate config, Prometheus rules and SLOs, and the `platform` repo.
- **Agents':** all Go code, the template's application code and its tests and configs, mise tasks, pre-commit config, `AGENTS.md` and docs.

## Consequences

- An agent never creates or edits a file on the author's side, even to make a test pass. Its ticket leaves the gap, and any test that needs the missing file skips with a message naming it.
- The author's tickets carry `ready-for-human`. The `guided-implement` skill coaches the author through them.
- The author can't review Go, so a human review isn't the merge gate for agent PRs. The gate is a review by a second model, every finding fixed or answered, and a passing `mise run check`. Once the author has written sow's CI, the required `ci-ok` check joins the gate.
- Agents of different strengths work on this repo. `docs/agents/coding-guide.md` and `docs/agents/workflow.md` hold the rules that keep their output consistent.
