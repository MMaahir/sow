# Coding guide

Rules for every agent that writes code in this repo. The author doesn't read Go, so nobody will catch a shortcut by eye. The rules below and the tests are what keep the code consistent. Follow them exactly, even when you'd choose differently.

The ticket says what to build. This guide says how. If they disagree on how, this guide wins. If you think a rule is wrong for your ticket, stop and say so in a comment on the ticket. Don't work around it.

## 1. Where code goes

| Path | What lives there | What never lives there |
|---|---|---|
| `cmd/sow/main.go` | Reads the real process (arguments, streams, environment, working directory, build version, whether stdin is a terminal) and calls `cli.Run` | Any logic. Don't add files to `cmd/sow` |
| `internal/cli/` | One file per command. Flags, argument checks, and one call into another package | File writing, rendering, starting processes |
| `internal/answers/` | The questions, presets, validation, the answers file and the personal defaults file | Anything about disk layout of a project |
| `internal/render/` | Turns template directories and answers into files. Implements the template format in ADR 0012 | Knowledge of specific answers or components |
| `internal/project/` | Work on a generated project on disk: creating it, follow-up steps, git | Flag parsing, printing to the terminal |
| `template/` | The files of generated projects | Go code, infrastructure files (see section 6) |
| `sow.go` | Embeds `template/` into the binary | Anything else |

Create a new package only when a ticket tells you to. A new file in an existing package is fine.

## 2. Recipes

Copy the existing code for each of these. Don't invent a second way.

### Add an answer

1. Add one `Question` to the list in `Questions()` in `internal/answers/answers.go`. Put it where it should appear in the answers file.
2. That's all the wiring. The `--flag`, the validation, the answers file entry and the template value `[[ .your_key ]]` all come from that entry.
3. Add tests in `internal/cli/new_test.go`: the default is recorded, the flag is recorded, a bad value is rejected and the error lists the choices.
4. Update the expected file in `TestNewWritesTheAnswersFile`.

Every answer is a string. A component answer has `Component: true`, and its value is a stack name or `none`. That holds for every component, including one with a single stack: `database` is `none` or `postgres`, never `true` or `false`.

### Add a stack to a component

1. Add the stack's name to that question's `Choices`. If the stack is TypeScript, add it to the question's `typeScript` list too. `Answers.TypeScriptOn` reads that list, and `sow new` then checks that Vite+ is set up before it writes anything (ADR 0011).
2. Create `template/<component>-<stack>/`, such as `template/backend-ts/`. `Answers.Dirs()` selects it by that name. Don't add a lookup table.
3. Put the stack's files inside it at the paths they should have in the project. `template/backend-ts/backend/package.json` becomes `backend/package.json`.
4. Give the stack a `mise.toml` in its own directory with the `dev`, `test`, `lint` and `build` tasks (ADR 0004), as `template/backend-ts/backend/mise.toml` does.
5. Shared files in `template/base/` that must know about the stack are `.tmpl` files that branch on the answer: the root `mise.toml`, `.pre-commit-config.yaml` and `AGENTS.md`. Add a branch there. Don't put a second copy of a shared file in the stack's directory.
6. Add fast tests with a fake tool (see "Start an external command"), like `internal/cli/backend_ts_test.go`, and health tests like `TestNewBackendTSProjectHealth`.
7. Prove the stack in a throwaway project outside the repo first, and record the versions and surprises in `docs/research/`, as `docs/research/backend-ts-prototype.md` does.

### Add a preset

Add one entry to `presets` in `internal/answers/presets.go`, and tests like `TestNewMinimalPresetTurnsNoComponentsOn`.

### Add a template file

1. If the file's content is the same for every project, add it as a plain file. It's copied byte for byte.
2. If the content needs an answer, add `.tmpl` to the file name and read the answer as `[[ .project_name ]]`. The key is the answer's key.
3. Add a render test that checks the file exists and contains what matters.

### Add a follow-up step

1. Add one `followUpStep` to the list in `followUpSteps` in `internal/project/steps.go`. Set `name` to what the step does, `file` to a path relative to the project root, and `run` to the function that does the work.
2. The step runs only if its file exists in the project, after the files are written and before the first commit. Its error is wrapped with `name`.
3. Test it in `internal/cli/` through the sandbox: the step's effect is on disk, and `git status` is clean after `sow new`. A file the step creates for the local machine only, such as `.env`, is gitignored and stays out of the first commit. A file the project needs, such as the lockfile `vp install` writes, is in the first commit.
4. Test the failure too: the CLI exits 1, stderr names the command and the directory, and the rendered files are still there.

### Add a command

1. Create `internal/cli/<command>.go` with a function `<command>Command(env Env) *cobra.Command`, shaped like `newCommand` in `internal/cli/new.go`.
2. Register it in `Run` in `internal/cli/cli.go` with `root.AddCommand`.
3. The command's `RunE` reads flags, then makes one call into `internal/project` or another package, and returns its error. `Run` prints the error.
4. Create `internal/cli/<command>_test.go` and test through the sandbox.

### Start an external command

Use `run` in `internal/project/new.go`. It finds the executable on the `PATH` of the environment it was given, sets the directory and environment, and returns an error that names the command, the directory and the output. Never call `exec.Command` or `exec.LookPath` anywhere else in non-test code, because they read the real process's `PATH`. Pass `opts.Environ` through, and never use `os.Environ()`.

To test a command sow starts, call `s.fakeTool("vp", script)` in the sandbox. It puts a shell script with that name first on the sandbox's `PATH`.

## 3. Go rules

Each rule has a reason. The rule still applies when the reason seems not to.

1. **Only `cmd/sow/main.go` touches the process.** No `os.Stdin`, `os.Stdout`, `os.Stderr`, `os.Getenv`, `os.Environ`, `os.Getwd`, `os.Chdir` or `os.Exit` anywhere else in non-test code. Read them from `cli.Env`, and pass down only the values a function needs. Tests run commands in-process and in parallel directories, and a direct read would see the real machine.
2. **Return errors. Never panic, never `log.Fatal`.** The one existing `panic` in `sow.go` guards a constant and is the only exception.
3. **Never drop an error.** Every returned error is checked. `_, _ = fmt.Fprintf(env.Stderr, ...)` is the only allowed discard, because nothing can be done when printing an error fails.
4. **Wrap errors with context using `%w`.** `fmt.Errorf("read the personal defaults file %s: %w", path, err)`. The message starts lowercase, has no trailing full stop, and says what sow was doing. Don't add context the inner error already has. `os` errors already contain the path.
5. **An error the author will see tells them what to do next.** `user_name is missing: pass --user-name or set user_name in the personal defaults file`. Name the flag, the file or the command.
6. **Report every problem at once when validating.** Collect errors and return `errors.Join(problems...)`, as `answers.Resolve` does. The author shouldn't fix one answer only to hit the next.
7. **Check before you write.** Validate and render everything in memory first, then write files, then start the commands that change the project. `project.New` does this. A command that only reads the machine, such as `vp env doctor`, may run before anything is written, so sow can refuse early. A failure in validation or rendering must leave nothing on disk. A failure after writing has started, such as a follow-up step that fails, leaves the files in place so the author can fix the cause and run the command again.
8. **No new dependencies.** The module uses cobra, `huh` (`charm.land/huh/v2`) for prompts and a YAML library. `cmd/sow/main.go` also imports `github.com/charmbracelet/x/term`, which huh already depends on, to check whether stdin is a terminal. Anything else needs a ticket that names it. The standard library covers the rest: `slices`, `maps`, `strings`, `errors`, `os`, `io/fs`, `path/filepath`, `text/template`.
9. **No package-level variables that change.** Package-level `var` is allowed only for values set once and never reassigned, such as a compiled `regexp` or the `presets` map. No `init()` functions.
10. **No interface until two real implementations exist.** Don't add one for testing. Tests use the real file system in a temporary directory and real `git`. The one exception is `cli.Prompter`, because a test has no terminal to type into. Its only real implementation is `cli.TerminalPrompter`, and the ticket that added prompts named this seam.
11. **No goroutines, channels, generics or reflection** unless the ticket asks for them.
12. **`context.Context` is the first parameter** of any function that starts a command or can take long. Pass the one you were given. Never call `context.Background()` outside `cmd/sow/main.go`.
13. **Use `filepath` for paths on disk and `path` for paths inside the template.** The template file system always uses forward slashes. Convert with `filepath.FromSlash` when writing.
14. **Every exported name has a doc comment** that starts with the name and says what it does and what it returns on failure. Add a "why" sentence when the code would surprise a reader. Comments describe the code as it is now. Don't write what changed, and don't mention tickets.
15. **Return early.** Handle the error or the special case, return, and keep the main path unindented. No `else` after a `return`.
16. **Name things after the glossary.** `answers`, `preset`, `component`, `stack`, `followUpStep`. Read `GLOSSARY.md` and never use a word from its "Avoid" lines. Receivers are one letter (`a Answers`, `e Env`). No `utils`, `helpers` or `common` packages or files.
17. **File modes are `0o755` for directories and `0o644` for files.**
18. **No `//nolint`.** Fix the code. If a linter is wrong, stop and comment on the ticket.
19. **Change only what the ticket needs.** Don't rename, reformat or reorganise code the ticket doesn't touch. If you see a problem elsewhere, list it in the PR summary as a noticed problem.
20. **Don't write code for later tickets.** No unused parameters, options, flags or files "for when we need them".

## 4. Test rules

### The seams

There are three places to test. Don't test anywhere else unless your ticket names a new seam.

- **`cli.Run`, through the sandbox** in `internal/cli/sandbox_test.go`. This is the default. A test creates a sandbox, runs `sow` with arguments, and checks the exit code, the output and the files. Everything in `answers` and `project` is tested this way.

  The sandbox replaces the template through `Env.Template`. `cmd/sow/main.go` passes the embedded template, and so does the sandbox, unless a test calls `s.install(version, files)`. That makes sow run as `version` with `files` as an in-memory template. For `sow update`, `s.release(version, files)` commits `files` under `template/` in a local fixture git repo, tags the commit `v<version>` and points `SOW_REPO` at the repo, so the update fetches the old template without the network. `s.releasedProject` does both and creates the project. The sandbox's default `SOW_REPO` is a path that doesn't exist, so a test that forgets `release` fails instead of reaching GitHub.

  The sandbox replaces the terminal through one seam, `Env.Prompter`. `cmd/sow/main.go` sets it only when stdin is a terminal, and the sandbox leaves it unset, so a plain `s.run` has no terminal. To answer prompts, call `s.terminal(map[string]string{"backend": "none"})` before the run. It returns a scripted prompter that answers by key and records each question in `asked`. A question the test didn't script acts as Ctrl+C.
- **`render.Render`, with an in-memory template** (`fstest.MapFS`), in `internal/render/render_test.go`. Use it only for rules of the template format, such as what happens when two directories contain the same path.

- **The template's source files**, in `template_test.go` at the repo root. Use it only for rules a rendered project can't show, such as "no file named `.env`" or "`package.json` is never a `.tmpl`". These tests read `template/` on disk with `templatePaths`, not the embedded copy, because Go's embed leaves some files out without an error.

Don't write tests for unexported functions, and don't write tests in `internal/answers` or `internal/project`. If something there can't be reached through `cli.Run`, it's either dead code or the ticket should name a seam.

### Writing a test

```go
func TestNewRefusesAnExistingDirectory(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/notes.txt", "mine\n")

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "demo already exists")
	if got, want := s.read("demo/notes.txt"), "mine\n"; got != want {
		t.Errorf("notes.txt = %q, want it untouched", got)
	}
}
```

1. **The name is a sentence about behaviour**: `Test<Command><What it does>`. Not `TestResolve`, not `TestNew2`.
2. **Three blocks separated by blank lines**: set up, one action, checks.
3. **Write the expected value as a literal.** Never compute it with the code under test or copy its logic. `want := "# demo\n"`, not `want := "# " + name + "\n"`.
4. **A failure test checks three things**: the exit code is 1, the message names the problem and the fix, and the state of the disk. For a failure before writing starts (bad answers, a template error), nothing was written. For a failed follow-up step, the files are still there.
5. **Use `s.mustRun` when the command should succeed** and `s.run` when you want to inspect a failure.
6. **Use the standard library only.** No assertion packages. Use `t.Errorf` for a failed check so the other checks still run, and `t.Fatal` only when the rest of the test can't continue.
7. **Several cases of one behaviour go in a map and `t.Run`**, as in `TestNewRejectsAnInvalidProjectName`. Each case gets a new sandbox.
8. **Helpers call `t.Helper()`** and belong in `sandbox_test.go`. Add a method to the sandbox before you repeat setup code in three tests.
9. **Tests never read the real machine**: no real home directory, no real git config, no network. The sandbox sets `HOME` and the git variables. A test that needs another variable sets `s.extraEnv`.
10. **Slow tests have `Health` in their name, live in `internal/cli/health_test.go`, and start with `newHealthSandbox`.**

    ```go
    func TestNewBackendTSProjectHealth(t *testing.T) {
    	s := newHealthSandbox(t, "mise", "vp", "node")
    	s.newProject("--backend", "ts")
    	s.mustTool(".", "mise", "install")

    	t.Run("lint, test and build pass in the backend", func(t *testing.T) {
    		s := s.in(t)
    		for _, task := range []string{"lint", "test", "build"} {
    			s.mustTool("backend", "mise", "run", task)
    		}
    	})
    }
    ```

    They run the generated project's own tasks with the real tools. `newHealthSandbox` skips the test in short mode and when a tool it names isn't installed, and it's the one sandbox that inherits the real machine's environment. Create the project once per stack and put each check in a `t.Run`, because installing dependencies is slow. Every subtest starts with `s := s.in(t)`, so a failure is reported to the subtest. A subtest that changes the project undoes the change in `t.Cleanup`. Run project tasks through `mise run`, as the author would.
11. **A health check that needs a file the author hasn't written yet skips itself** with a message naming the file: `t.Skip("backend/Dockerfile doesn't exist yet: the author writes it")`. Check for the file in the rendered project, not in `template/`.

### Red, then green

Work one behaviour at a time.

1. Write one test. Run `mise run test`. Watch it fail, and read the failure to confirm it fails for the reason you expect. A test that passes before you write the code is testing nothing new, so fix the test.
2. Write the smallest code that makes it pass.
3. Run `mise run check`.
4. Repeat for the next behaviour.

Never change a test to make it pass unless the test itself was wrong, and say so in the PR. Never delete a failing test, and never add `t.Skip` to one. The only skips allowed are the two in test rules 10 and 11.

## 5. Template rules

ADR 0012 defines the template format. It's a compatibility contract, because `sow update` renders old releases with the current binary. Don't change how rendering works unless your ticket says so.

1. **Add `.tmpl` only when the file needs an answer.** Plain files are safer and other tools can parse them. Keep `package.json`, `pnpm-workspace.yaml` and lockfiles plain, so Renovate can read them.
2. **Delimiters are `[[` and `]]`.** `{{ }}` passes through untouched.
3. **Templates only substitute answers and branch on them**: `[[ .project_name ]]` and `[[ if eq .backend "ts" ]]...[[ end ]]`. No loops, no variables, no nested templates, no custom functions. If you need more, the file is doing too much. Stop and comment on the ticket.
4. **Put `[[- if ... ]]` and `[[- end ]]` on their own lines, with the dash on the left only.** `[[-` removes the whitespace before the action, which is the previous line's newline, so a skipped branch leaves no blank line and the lines inside the branch keep their indentation. Never write `-]]`, because it also removes the indentation of the next line. Check the rendered output in a test, as `TestRenderABranchOnItsOwnLinesLeavesNoBlankLines` does.

   ```
   before:
   [[- if eq .backend "ts" ]]
     middle
   [[- end ]]
   after
   ```

   This renders `before:`, `  middle`, `after` on three lines when the backend is `ts`, and `before:`, `after` on two lines otherwise. `[[-` removes every blank line before it too, so a blank line that belongs to the branch goes inside it, as the first line after the `if`. Don't start a file with a branch.

5. **A path belongs to one directory.** If two directories that can be selected together contain the same path, rendering fails. A file that depends on several answers lives in `template/base/` as a `.tmpl`. The one exception is `template/typescript/`: it holds the plain files every TypeScript stack shares, such as the root `package.json` and `pnpm-workspace.yaml`, which rule 1 keeps plain. sow selects it when any TypeScript stack is on. Never copy those files into a stack's own directory.
6. **Never add a file named `go.mod` under `template/`.** Go's embed skips any directory that contains one. Name it `go.mod.tmpl`.
7. **Files aren't executable.** The embedded file system drops the executable bit. Call scripts through their interpreter, such as `sh scripts/foo.sh`.
8. **The template never contains `.env`.** It contains `.env.example`.
9. **Generated code follows the same standard as sow's code**: it has tests, it passes its own lint, and it has no placeholder comments or TODOs.
10. **mise tasks are the interface.** Every stack provides `dev`, `test`, `lint` and `build` tasks (ADR 0004). TypeScript stacks follow ADR 0011 exactly, including which tool owns Node and pnpm.

## 6. Infrastructure files belong to the author

Never create or edit these, in this repo or under `template/` (ADR 0013):

- `.github/workflows/*`, and any other GitHub Actions file
- `Dockerfile`, `.dockerignore`, `compose.yaml` and other Compose files
- Helm charts, Argo CD manifests, Kubernetes manifests
- Terraform (`infra/`)
- Renovate config (`renovate.json`)
- Prometheus rules, alert rules and SLO definitions
- release configuration (release-please, GoReleaser)

If your ticket needs one of them to exist, build everything else, make the dependent test skip (test rule 11), and list the gap in the PR summary.

## 7. Writing prose

For comments, docs, commit messages, PR descriptions and issue comments.

- Write whole sentences in plain words. Use active voice and name the actor: "Render copies the file", not "the file is copied".
- State facts about what the code does. Cut any sentence that could appear unchanged in another project.
- No em dashes. No bold text for emphasis. No emojis. Sentence case for headings.
- Don't use a colon to join two clauses. Use it only before a list or an example.
- Don't write "not just X but Y", don't group things in threes for rhythm, and don't end with a summary sentence.

## 8. Before you commit

Go through this list every time. Don't commit until each line is true.

- [ ] `mise run fmt` ran, and `mise run check` passes with no warnings.
- [ ] If you touched `template/` for a stack, `mise run test-health` passes or skips only for files the author owns.
- [ ] Every acceptance criterion in the ticket has a test that fails without your change.
- [ ] You saw each new test fail before you made it pass.
- [ ] No infrastructure file is created or changed (section 6).
- [ ] No new dependency in `go.mod` that the ticket didn't name.
- [ ] No file changed that the ticket doesn't need.
- [ ] Every exported name you added has a doc comment.
- [ ] The docs still match: `docs/roadmap.md`, `GLOSSARY.md`, `README.md` and this guide. If you added a pattern other agents should copy, add a recipe to section 2.
