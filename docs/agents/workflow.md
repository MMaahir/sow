# Workflow

How an agent picks up a ticket, builds it, gets it reviewed and merges it. Any agent can start from here with no other context. All state lives in GitHub issues and git, so there's no progress file to keep up to date.

## 1. How the work is organised

- A **spec** is a GitHub issue titled `Spec 0.N: ...`. It describes one release.
- A **ticket** is a sub-issue of a spec. It's one slice of work that fits in one session.
- A ticket lists the tickets that block it under "Blocked by". It's ready when every blocker is closed.
- Labels say who does the ticket and how hard it is.

| Label | Meaning |
|---|---|
| `ready-for-agent` | An agent builds it |
| `ready-for-human` | The author builds it by hand. Agents never touch it (ADR 0013) |
| `difficulty:hard` | Design choices are left open, or it changes the template format, update logic or a seam. Use the strongest model available |
| `difficulty:medium` | The design is settled but the ticket touches several packages or needs outside research |
| `difficulty:easy` | Follows a recipe in the coding guide. A small, fast model can build it |

If you're a small model and the ticket isn't `difficulty:easy`, stop and tell the author. Don't attempt it.

## 2. Pick a ticket

```sh
gh issue list --state open --label ready-for-agent \
  --json number,title,labels,assignees,body \
  --jq '.[] | select(.assignees | length == 0) | "\(.number)\t\(.title)\t\([.labels[].name] | join(","))"'
```

1. Skip issues titled `Spec ...`. They're parents, not work.
2. Take the lowest release first, then the lowest issue number.
3. Read the ticket: `gh issue view <n> --json number,title,body,labels,comments`. Read the comments too. A paused ticket has a handoff comment (section 7).
4. Check each issue under "Blocked by" with `gh issue view <blocker> --json state,labels`.
   - Every blocker is closed: the ticket is ready.
   - An open blocker is labelled `ready-for-agent`: the ticket isn't ready. Pick another.
   - An open blocker is labelled `ready-for-human`: the ticket is ready only if its body says which checks skip until the author's file exists. Build the rest and make those checks skip (coding guide, test rule 11). If the body doesn't say so, the ticket isn't ready.
5. Claim it: `gh issue edit <n> --add-assignee @me`.

## 3. Prepare

1. Read `AGENTS.md`, `docs/agents/coding-guide.md`, `GLOSSARY.md`, the ticket's parent spec, and each ADR the ticket or spec names.
2. Run `mise install`, then `mise run check` on `main`. If it fails before you change anything, stop and report it.
3. Branch from an up-to-date `main`: `git switch -c <type>/<issue>-<slug> origin/main`, for example `feat/41-always-on-files`. The type is a Conventional Commits type.

## 4. Build

1. Turn the acceptance criteria into a list of behaviours. Order them so each one builds on the last.
2. For each behaviour, follow "Red, then green" in the coding guide: one failing test, the smallest code that passes, `mise run check`.
3. Commit after each behaviour that passes. Messages follow Conventional Commits: `feat: reject an unknown preset`.
4. When every criterion is met, go through "Before you commit" in the coding guide one more time.

If you can't meet a criterion, don't fake it and don't weaken the test. Stop, and write a handoff comment (section 7).

## 5. Open the PR

```sh
git push -u origin HEAD
gh pr create --title "<conventional commit title>" --body-file <file>
```

The title becomes the squash commit, so it follows Conventional Commits. The body has these sections:

- **What changed**, in plain words the author can follow without reading Go.
- **How to try it**: commands the author can paste to see the behaviour.
- **Left for the author**: every infrastructure file the change needs and doesn't have, and which checks skip until it exists.
- **Noticed**: problems you saw outside the ticket and didn't fix.
- `Closes #<ticket>`

## 6. Review and merge

The author doesn't read Go. The review by a second model is the gate, so it's never skipped and the author of the code never reviews their own PR.

1. **Ask a different model to review.** Use the strongest reviewer available that didn't write the code. The author's setup uses `gpt-6.1-sol` for this. Give the reviewer:
   - the diff command: `git diff origin/main...HEAD`
   - the ticket body and the parent spec number
   - `docs/agents/coding-guide.md` and the ADRs the ticket names
   - this instruction: "Review for two things. Spec: does the code do what the ticket's acceptance criteria say, and is each criterion covered by a test that would fail without the change? Standards: does it follow the coding guide? Also report real bugs. Report each finding with file and line, the axis, and the reason. Don't edit files."
2. **Answer every finding.** For each one, read the code it points at and decide:
   - It's right. Fix it, with a test if it's a behaviour bug.
   - It's wrong. Say why, with evidence: the line of the guide, the ADR, or a test run that shows the behaviour. Send the rebuttal back to the reviewer and let it answer once.
   - You still disagree after its answer. Don't merge. Write both positions in a PR comment and leave the PR for a stronger model or the author.

   A reviewer can be wrong. Don't change correct code because a reviewer asked. Don't dismiss a finding because fixing it is work.
3. **Record the review.** Post a PR comment listing each finding and what happened to it: fixed, rebutted and withdrawn, or open.
4. **Merge when all of these are true:** no finding is open, `mise run check` passes on the final commit, `mise run test-health` passes or skips only for the author's files, and the branch is up to date with `main`. If the repo has a required `ci-ok` check, it must be green too.

   ```sh
   gh pr merge <n> --squash --delete-branch
   ```
5. Confirm the ticket closed. If the spec has no open tickets left, comment on the spec that it's complete, and leave closing it to the author.

Several tickets built in one session can share one integration branch and one PR, as the `implement-spec` skill describes. The same gate applies to that PR.

## 7. Stopping partway

You may run out of budget or hit something you can't solve. Leave the work so the next agent loses nothing.

1. Commit only what passes `mise run check`. Don't commit a failing test and don't add `t.Skip` to one. Discard it, and describe it under "Next step" so the next agent writes it again.
2. Push the branch.
3. Comment on the ticket:

   ```
   Paused.
   Branch: <branch>
   Done: <criteria met, each with its test name>
   Not done: <criteria left>
   Next step: <the exact next thing to do>
   Problem: <what stopped you, with the command and its output, or "none">
   ```
4. Remove yourself: `gh issue edit <n> --remove-assignee @me`.

To resume a paused ticket, check out its branch, merge `origin/main` into it, run `mise run check`, and continue from "Next step".

## 8. Which model does what

The author's routing. A session that orchestrates others follows it.

| Work | Model |
|---|---|
| `difficulty:hard` tickets, planning, specs, answering review findings | The strongest model (Claude Fable) |
| `difficulty:medium` tickets | Claude Opus |
| `difficulty:easy` tickets | GLM-5.3-Flash |
| Code review | GPT-6.1-Sol, never the model that wrote the code |

When a stronger model hands an easy ticket to a smaller one, the brief is the ticket number plus "follow `docs/agents/workflow.md`". Don't restate the ticket. If the ticket isn't clear enough for that, fix the ticket.

## 9. Writing tickets

A ticket a small model can build has all of these:

- One behaviour slice, with a "What to build" paragraph written from the author's point of view.
- Acceptance criteria that are each checkable by one test. Give literal expected values: exact file names, exact messages, exact exit codes.
- The recipe in the coding guide it follows, by name. If no recipe fits, the ticket is `difficulty:medium` or harder.
- Its blockers, and which of them are the author's.
- No file paths or code that will go stale, except names the criteria depend on.
