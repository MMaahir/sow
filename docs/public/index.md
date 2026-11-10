# sow

sow is a command-line tool that creates a TypeScript or Go web project from one template. A new project starts with CI, linting, tests, agent skills config and mise tasks already set up. `sow update` later brings template changes into projects that already exist.

sow is a personal tool, and it's still before 1.0. Commands and answers can change between releases.

## Commands

| Command | What it does |
|---|---|
| `sow new <name>` | Creates a project from a preset or from answers given as flags |
| `sow update` | Updates a project to the template of the installed sow version |
| `sow add <component> [<stack>]` | Turns on a component in an existing project |
| `sow doctor` | Checks your tools and whether the project is behind the installed sow version |

`sow new` is the only command in the current release. The others arrive in later releases.

## Next step

[Install sow](install.md).
