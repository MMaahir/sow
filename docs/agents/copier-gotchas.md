# Copier gotchas

Behaviour of Copier 9.19.1, Jinja and the test harness that the docs don't state plainly. Each one was confirmed by experiment while building `sow new` (#11).

## Questions and answers

- Copier records only **declared questions** in `_copier_answers`. A value passed through `run_copy(data=...)` with no matching question in `copier.yml` still renders, but it's missing from `.copier-answers.yml`, so `sow update` couldn't replay it. Declare every answer you pass.
- The answers file is written only if the template has `{{ _copier_conf.answers_file }}.jinja` rendering `{{ _copier_answers|to_nice_yaml }}`.
- A **validator** renders nothing for a valid answer and an error message otherwise. On failure, `run_copy` raises `ValueError("Validation error for question '<name>': <message>")` before writing any file, so the destination directory is never created. The CLI catches it, prints it to stderr and exits 1.

## Jinja and regexes

- The Ansible core filters, including `regex_search`, are always loaded. They need no `_jinja_extensions` entry, so they don't break ADR 0010's "no Jinja extensions" rule.
- Jinja applies backslash escapes in string literals before the regex sees them. Write `\\Z` to get `\Z`. An unknown escape such as `\-` or a single `\Z` raises a `DeprecationWarning`, which the suite turns into a failure. A hyphen goes last in a character class, with no escape: `[a-z0-9-]`.
- In Python regexes, `$` also matches just before a final newline, so `^[a-z]+$` accepts `"demo\n"`. End anchors with `\Z`.

## Rendering from the local checkout

- With uncommitted template changes, Copier warns `DirtyLocalWarning` and renders from a throwaway commit. `_commit` then names a hash that doesn't exist in the repo. The suite ignores this warning, and the value is right once the changes are committed.
- With no tags in the repo, Copier reports the template version as `0.0.0.post<N>.dev0+<hash>`.

## Tests

- Typer's `CliRunner` mixes stderr into `result.output`, so an error printed with `typer.echo(..., err=True)` shows up there.
- `tests/conftest.py` writes the test gitconfig into `tmp_path`, so `tmp_path` is never empty. To check that nothing was created, assert `not (tmp_path / name).exists()`.
- `git init -b main` overrides `init.defaultBranch`. The test gitconfig sets `defaultBranch = master` to prove it.
