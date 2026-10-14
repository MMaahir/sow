# Agent skills project: reference notes

Notes for the agent skills repo, gathered while planning sow. Read this before you start that project. Researched 2026-10-13 and 2026-10-14. Codex changes often, so check the Codex facts again before relying on them.

## What sow needs from this project

sow depends on a contract, not on what the skills say:

- One plugin marketplace (this repo) holding several plugins split by stack, for example `core`, `python`, `typescript`, `platform`.
- Each generated project enables only the plugins that fit the components chosen for it.
- Claude Code gets the enablement through `.claude/settings.json`: `extraKnownMarketplaces` points at this repo and `enabledPlugins` lists the plugins.
- Codex installs plugins per user, not per project: `codex plugin marketplace add <owner>/<repo>`, run once per machine. Generated projects don't configure Codex. `sow doctor` checks that the marketplace is installed.
- Until this repo exists, sow adds `mattpocock/skills` through `extraKnownMarketplaces` and enables `mattpocock-skills@mattpocock` in its place.

Priority: build for Claude Code first. Codex only has to work.

## Reference layout: mattpocock/skills

https://github.com/mattpocock/skills serves Claude Code, Codex and Copilot from one repo. Worth copying:

- **Manifests.** `.claude-plugin/plugin.json` declares the plugin and lists each published skill directory in a `skills` array. `.claude-plugin/marketplace.json` makes the repo its own marketplace, with the plugin's `source` set to `"./"`.
- **Status folders.** Skills live under `skills/<bucket>/<name>/SKILL.md`. Skills in `engineering/` and `productivity/` are published. Skills in `in-progress/`, `misc/` and `deprecated/` stay in the repo but are left out of the `skills` array, so they don't ship.
- **Codex compatibility.**
  - With no `.codex-plugin/` present, Codex reads `.claude-plugin/`. It has accepted the `skills` array since about 2026-10-07.
  - Codex-only metadata goes in `skills/<bucket>/<name>/agents/openai.yaml`.
- **Releases.**
  - Changesets and `changesets/action` open a version PR.
  - `scripts/sync-plugin-version.mjs` copies the `package.json` version into `plugin.json`, and has a `--check` mode for CI.
  - Claude Code compares the plugin `version` to decide when to update installed copies, so a mismatched version means users don't get the update.
- **Local development.** `scripts/link-skills.sh` symlinks each skill into `~/.claude/skills` and `~/.agents/skills`, so a `git pull` updates the local copies.
- **Decision records.** The repo records its decisions in ADRs under `.agents/adr/`. `0002-ship-as-a-claude-code-plugin.md` explains how Codex support developed.

## Constraints for working in both agents

- Skill format: a `SKILL.md` with `name` and `description` frontmatter (the Agent Skills standard, agentskills.io). Claude-only frontmatter keys, such as `disable-model-invocation`, `context` and `paths`, may not work in Codex. Use them only in skills you don't need in Codex.
- Codex drops symlinks when it installs a plugin, so every skill in the plugin has to be a real file.
- Codex reads skills scoped to a repo from `.agents/skills/`. Claude Code reads them from `.claude/skills/`. Distributing skills as a plugin avoids having to maintain both.
- Claude Code only applies a repo's `extraKnownMarketplaces` after the user trusts the workspace. A plugin whose marketplace entry points at another repo or at npm doesn't install automatically. Use relative `./` sources so plugins install automatically.
- Validate the plugin with `claude plugin validate . --strict`.

## Per user or per project

Decided 2026-10-14: both.

- Enable the `core` plugin per user, so repos that sow didn't create still get it.
- Each generated repo declares every plugin it needs in `.claude/settings.json`, `core` included. This makes Claude Code cloud sessions and CI agents work, because they only see what the repo declares.
- `enabledPlugins` is keyed by `plugin@marketplace`, so a plugin enabled both per user and per repo loads once.
- Codex only installs per user, so none of this applies to Codex.
