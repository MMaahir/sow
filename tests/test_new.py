import os
import pathlib

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
