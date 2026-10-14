# One Copier template, with components turned on by answers

Generated projects come from a single Copier template. Its `copier.yml` sits at the root of this repo with `_subdirectory: template`, so a tag on this repo, such as `v0.1.0`, is both the CLI's version and the template's version. Answers like `backend: none | python | ts` and `database: true` turn components on. Each stack lives in its own conditional directory, such as `{% if backend == "python" %}backend{% endif %}`, so most files contain no `{% if %}`. Only shared files like `AGENTS.md`, `.claude/settings.json`, `.env.example`, the Compose file and the CI workflows branch on answers.

This replaces layered templates (ADR 0001), split mirrors (ADR 0002) and file owners (ADR 0003). Combining several templates needed split mirrors to get tags, an owner for every shared file, an ordered update across several answers files with recovery when one failed partway, and rules for which layers could combine. With one template, Copier handles all of that: one answers file, one `copier update`, and prerequisites expressed as `when` conditions and choice validators. The cost is ADR 0001's worry that conditionals make one template hard to read. Conditional directories keep that manageable for the two or three stacks each component will have.

## Consequences

- Adding a component is an update with one answer changed: `--data database=true` for an on-or-off component, or `--data backend=python` for a component with stacks. If the component is already on, `sow add` exits without changes. Switching an existing component to another stack isn't supported. Turning a component off would delete its unedited files, but no `remove` command is planned.
- Projects generated from 0.1 can be updated, because Copier records `_src_path` and `_commit` from this repo's tags.
- Tags must be plain `vX.Y.Z`. Copier reads the version through dunamai's default pattern, so release-please runs with `include-component-in-tag: false`.
- Never move a tag after a release.
