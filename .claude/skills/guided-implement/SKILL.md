---
name: guided-implement
description: Coach the user through implementing a ticket: you write the tests, the user writes the code. Use when the user asks to go through, work through or pair on a ticket together ("let's go through this ticket together", "walk me through #12"), or asks for help working on a ticket.
---

# Guided implement

This is `/implement` with the roles split: you write the tests, and the user, who is learning, writes every line of source and config to make them pass. You _coach_ the implementation.

Rules for every step:

- Write and fix the test files yourself. Leave source, template and config files to the user. Show patterns in chat; the user types them into the files.
- Give the reason with every suggestion: why this test, why this seam, why this change.
- Take one step, or one beat inside a step, then end your turn and wait for the user.
- Write every message for a learner: open with a line or two on where you are and what this step is for, then use short sentences with one idea each, define each new concept before you use it, and put what to do in a numbered list.
- Give docs just in time. When the user is about to act (install a dependency, configure a tool, write a test, make it pass, run a check), link what that action needs, and hold links for later beats until those beats arrive. Give one source per thing they need now, linked to the section rather than the homepage, with a line on what to read there and how it applies. Put each link inline, in the instruction it supports, never in a separate reading section. Prefer the tool's own docs page or section, then the spec or PEP, then one well-regarded blog post where the docs are thin.
- Open every link with WebFetch before you give it, and confirm it says what you claim. Fetch each page once and check every anchor you need from that one fetch. Keep the verified links in the plan, so later beats reuse them after compaction. Where the docs are silent and you checked the behaviour by experiment, say so.
- Put every question and decision to the user through the `mattpocock-skills:grilling` skill, in its round format. Write no docs for the answers: no ADR, glossary entry or file. Keep them in the thread and carry each settled answer into the plan.

## 1. Load the ticket

Fetch the ticket as `docs/agents/issue-tracker.md` describes and state its number and title. If any blocker is still open, say which and stop. Read `GLOSSARY.md`, the ADRs the ticket touches, and the roadmap section for its release.

Restate the ticket's acceptance criteria as a checklist. Create a branch off an up-to-date `main`, named `<type>/<issue>-<slug>` with a Conventional Commits type (`feat/12-sow-new`), and record the `main` SHA as the review's fixed point.

Done when the user has the checklist and the branch is checked out.

## 2. Seams and slices

Invoke the `mattpocock-skills:tdd` skill and use it as the reference for steps 2 and 3. Propose the seams as `tdd` describes, and an ordered list of vertical slices, each a tracer bullet. Grill the user on the seams, the slices and any design decisions the ticket leaves open.

Done when the grilling frontier is empty and the plan lists the confirmed seams, the slices and every settled decision.

### Offline sessions

If the user will be offline for the slices, you can't coach them live, so prepare everything before they disconnect:

1. Write the tests for every slice up front, in slice order, and run them to confirm they fail for the right reason.
2. Prove the tests can pass. Copy the repo to a temp dir, run `uv sync --offline` there so the editable install points at the copy, write a throwaway implementation, and run the suite, linter and type checker. Fix any test that can't go green, then delete the copy. Never put the implementation in the user's checkout.
3. Write a notes file outside version control (`.notes/<issue>.md`, listed in `.git/info/exclude`). It holds the settled decisions, each slice's tests and the smallest change that makes them pass, the commands to run, and the facts the user would otherwise look up online: quote the docs or cite library source in `.venv` with `file:line`, since links won't open.
4. Then do steps 4–6 once the user is back online.

## 3. Coach each slice, red then green

For each slice, take these beats in order:

1. **Behaviour.** Say in one sentence what the test proves. Name the independent source of the expected value: a literal, a worked example, or a line of the ticket.
2. **Test.** Write the test, checked against the `tdd` anti-patterns and its `mocking.md`. Walk the user through its arrange, act and assert, and name the pytest tools it uses (fixtures, `tmp_path`, `parametrize`, Typer's `CliRunner`) with a link to each.
3. **Red.** Run the test and show the failure. Confirm the missing behaviour causes it, rather than a typo or a broken fixture, and explain what the failure tells the user to build.
4. **Green.** Say where the code goes and what the smallest change is, and link the docs for any dependency, tool setup or library call it needs. When the user is stuck, give more help one rung at a time: the idea, then the doc section, then a short snippet in chat.
5. **Checks.** The user runs the test file, the type checker and the linter. On a tool's first run, or a failure new to the user, link the docs section for it.
6. **Commit.** Suggest a Conventional Commits message for the slice. The user commits the test and the code together.

If the user's code passes but a test turns out to be wrong, fix the test yourself and say what was wrong with it.

Done when every slice is committed and the user's run of the full suite is green.

## 4. Review in two passes

1. Run the `mattpocock-skills:code-review` skill (not the built-in `code-review`) against the fixed point from step 1. Keep its findings for item 3 below.
2. Delegate an independent review with `delegate_task`:
   - `target`: `{"providerInstanceId": "codex", "model": "gpt-6.1-sol", "options": {"reasoningEffort": "xhigh"}}`
   - `role`: `review`, `mode`: `async`, `clientRequestId`: `guided-review-<issue>-<round>`
   - The prompt carries the diff command, the commit list, the ticket body, the standards files, the smell baseline from `code-review`, and the `tdd` anti-patterns. It asks for a read-only report, each finding with `file:line`, the axis (Spec or Standards), and the reason. Leave your own findings out so the second pass is independent.
3. Check every GPT finding against the code and drop the wrong ones. Then present one list:
   - **Must change**: missing or wrong spec behaviour, bugs, and breaches of documented standards.
   - **Worth changing**: judgement-call smells and refactors.
   - **Dropped**: findings you rejected, with a one-line reason each.

   Each kept item gives `file:line`, the axis, who found it, what's wrong, and the change to make.

The user makes and commits the changes, and you review them.

Done when Must change is empty.

## 5. Close out

1. Push the branch and open the PR with `gh pr create`. The title follows Conventional Commits, since it becomes the squash commit. Write the body with the `mattpocock-skills:pr` skill and put `Closes #<issue>` in it.
2. Watch the PR with `watch_pull_request`. When CI fails, coach the user through the fix the same way as step 3.
3. Once `ci-ok` passes, hand back. The user does the squash merge, which closes the issue.

Done when `ci-ok` is green and the PR is waiting for the user's merge.

## 6. Retro

Ask the user to run `/mattpocock-skills:retro` on this session before clearing it. The skill blocks model invocation, so you can't start it yourself. When it runs, follow it as written.

Done when the user has run the retro or declined it.
