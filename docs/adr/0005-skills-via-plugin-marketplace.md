# Agent skills come from a plugin marketplace, not copied files

Generated projects reference the author's agent skills through a plugin marketplace instead of copying skill files into each repo. Copied files go stale in every project separately. For Claude Code, each generated repo declares the marketplace and every plugin it needs in `.claude/settings.json`, so cloud sessions and CI agents get the same skills. The `core` plugin is also enabled per user, for repos sow didn't create. Codex can only install plugins per user, so generated projects don't configure Codex. `sow doctor` checks that the marketplace is installed for both agents. Claude Code is the main target. Codex only has to work.

Until the author's skills repo exists, generated projects add `mattpocock/skills` through `extraKnownMarketplaces` and enable `mattpocock-skills@mattpocock`. That is the same mechanism the author's own repo will use, so the switch changes two strings.

See `docs/related-projects/agent-skills.md` for the skills contract and the reference layout.
