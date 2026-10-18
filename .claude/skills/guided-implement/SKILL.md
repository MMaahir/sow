---
name: guided-implement
description: Coach the user through implementing a ticket themselves. Use when the user asks to go through, work through or pair on a ticket together ("let's go through this ticket together", "walk me through #12"), or asks for help working on a ticket.
---

# Guided implement

This is `/implement` with the roles swapped: the user is learning, so they write every test and every line of code, and you _coach_.

Rules for every step:

- Leave source, test and config files to the user. Show patterns in chat; the user types them into the files.
- Give the reason with every suggestion: why this test, why this seam, why this change.
- Take one step, or one beat inside a step, then end your turn and wait for the user.
- Give docs just in time. When the user is about to act (install a dependency, configure a tool, write a test, make it pass, run a check), link what that action needs, and hold links for later beats until those beats arrive. Give one source per thing they need now, linked to the section rather than the homepage, with a line on what to read there and how it applies. Put each link inline, in the instruction it supports, never in a separate reading section. Prefer the tool's own docs page or section, then the spec or PEP, then one well-regarded blog post where the docs are thin.
- Open every link with WebFetch before you give it, and confirm it says what you claim. Where the docs are silent and you checked the behaviour by experiment, say so.
- Put every question and decision to the user through the `mattpocock-skills:grilling` skill, in its round format. Write no docs for the answers: no ADR, glossary entry or file. Keep them in the thread and carry each settled answer into the plan.

## 1. Load the ticket

Fetch the ticket as `docs/agents/issue-tracker.md` describes and state its number and title. If any blocker is still open, say which and stop. Read `GLOSSARY.md`, the ADRs the ticket touches, and the roadmap section for its release.

Restate the ticket's acceptance criteria as a checklist. Create a branch off an up-to-date `main`, named `<type>/<issue>-<slug>` with a Conventional Commits type (`feat/12-sow-new`), and record the `main` SHA as the review's fixed point.

Done when the user has the checklist and the branch is checked out.

## 2. Seams and slices

Invoke the `mattpocock-skills:tdd` skill and use it as the reference for steps 2 and 3. Propose the seams as `tdd` describes, and an ordered list of vertical slices, each a tracer bullet. Grill the user on the seams, the slices and any design decisions the ticket leaves open.

Done when the grilling frontier is empty and the plan lists the confirmed seams, the slices and every settled decision.

## 3. Coach each slice, red then green

For each slice, take these beats in order:

1. **Behaviour.** Say in one sentence what the test proves, and suggest a test name. Name the independent source of the expected value: a literal, a worked example, or a line of the ticket.
2. **Test shape.** Describe arrange, act and assert in prose. Name the pytest tools that fit (fixtures, `tmp_path`, `parametrize`, Typer's `CliRunner`) with a link to each. The user writes the test.
3. **Red.** The user runs the single test file. Read the failure with them and confirm the missing behaviour causes it, rather than a typo or a broken fixture.
4. **Test review.** Read the test file and check it against the `tdd` anti-patterns and its `mocking.md`. Explain each problem and why it makes the test weaker. The user fixes it.
5. **Green.** Say where the code goes and what the smallest change is, and link the docs for any dependency, tool setup or library call it needs. When the user is stuck, give more help one rung at a time: the idea, then the doc section, then a short snippet in chat.
6. **Checks.** The user runs the test file, the type checker and the linter. On a tool's first run, or a failure new to the user, link the docs section for it.
7. **Commit.** Suggest a Conventional Commits message for the slice. The user commits.

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
