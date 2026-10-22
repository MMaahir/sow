import os
import pathlib
import subprocess

import pytest
import yaml
from typer.testing import CliRunner

from sow.cli import app


def test_new_renders_the_template_into_a_directory_named_after_the_project(
    tmp_path: pathlib.Path,
):
    """Test that 'sow new <project-name>' generates a new project in a directory named after
    the project. The directory's README.md must start with the line '# demo'.
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


@pytest.mark.parametrize("name", [pytest.param("a" * 53, id="53-characters"), "a"])
def test_new_accepts_a_valid_project_name(name: str, tmp_path: pathlib.Path):
    """Test 'sow new <project-name>' accepts a valid project name."""
    result = CliRunner().invoke(app, ["new", name, "--no-input"])
    assert result.exit_code == 0, result.output
    assert (tmp_path / name).is_dir()


@pytest.mark.parametrize(
    "name",
    [
        pytest.param("a" * 54, id="54-characters"),
        "demo-",
        "1demo",
        "Demo",
        pytest.param("demo\n", id="trailing-newline"),
    ],
)
def test_new_rejects_an_invalid_project_name(name: str, tmp_path: pathlib.Path):
    """Test 'sow new <project-name>' rejects an invalid project name."""
    result = CliRunner().invoke(app, ["new", name, "--no-input"])
    assert result.exit_code != 0, result.output
    assert "lowercase letters, digits and hyphens" in result.output
    assert not (tmp_path / name).exists()
