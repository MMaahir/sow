import os
import pathlib
import subprocess

import yaml
from typer.testing import CliRunner

from sow.cli import app


def test_new_renders_the_template_into_a_directory_named_after_the_project(
    tmp_path: pathlib.Path,
):
    """Test that 'sow new <project-name>' generates a new project in a new directory named after
    project, the directory should contain a README.md that starts with the line '# demo'.
    """
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    readme = tmp_path / "demo" / "README.md"
    assert result.exit_code == 0, result.output
    assert readme.read_text(encoding="utf-8").splitlines()[0] == "# demo"


def test_new_records_the_answer_in_the_answers_file(tmp_path: pathlib.Path):
    """Test that '.copier-answers.yml' file contains the answers when 'sow new <project-name>'
    is called.
    """
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    answers_path = tmp_path / "demo" / ".copier-answers.yml"
    assert result.exit_code == 0, result.output
    answers = yaml.safe_load(answers_path.read_text(encoding="utf-8"))
    assert answers["project_name"] == "demo"
    assert answers["_src_path"] == os.environ["SOW_TEMPLATE_SRC"]
    assert answers["_commit"]


def test_new_commits_the_project_as_the_first_commit(tmp_path: pathlib.Path):
    """Test that the created project is a git repository with exactly one commit, the commit
    message and the clean tree.
    """
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    assert result.exit_code == 0, result.output
    demo_path = tmp_path / "demo"
    log = subprocess.run(
        ["git", "log", "--format=%s"],
        capture_output=True,
        text=True,
        check=True,
        cwd=demo_path,
    )
    assert log.stdout.splitlines() == ["chore: initial commit from sow"]
    status = subprocess.run(
        ["git", "status", "--porcelain"],
        capture_output=True,
        text=True,
        check=True,
        cwd=demo_path,
    )
    assert status.stdout == "", status.stdout


def test_new_puts_the_first_commit_on_main(tmp_path: pathlib.Path):
    """Test that the first commit is on 'main', even when the user's git defaults to
    'master'.
    """
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    assert result.exit_code == 0, result.output
    demo_path = tmp_path / "demo"
    branch = subprocess.run(
        ["git", "branch", "--show-current"],
        capture_output=True,
        text=True,
        check=True,
        cwd=demo_path,
    )
    assert branch.stdout.strip() == "main"
