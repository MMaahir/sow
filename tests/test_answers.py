import pathlib
from typing import Any

import pytest
import yaml
from typer.testing import CliRunner

from sow.cli import app

# The start of each question's `help` in copier.yml. Copier prints the help text as the prompt.
USER_NAME_HELP = "Your name"
BACKEND_HELP = "Backend stack"

PERSONAL_FLAGS = {
    "user_name": ["--user-name", "Ada Lovelace"],
    "user_email": ["--user-email", "ada@example.com"],
    "github_user": ["--github-user", "ada"],
}


def read_answers(project: pathlib.Path) -> dict[str, Any]:
    return yaml.safe_load((project / ".copier-answers.yml").read_text(encoding="utf-8"))


# Personal defaults from the Copier settings file


def test_new_records_personal_defaults_from_the_copier_settings_file(
    tmp_path: pathlib.Path, copier_settings: pathlib.Path
):
    """Test that user_name, user_email and github_user come from Copier's settings file."""
    copier_settings.write_text(
        "defaults:\n"
        "  user_name: Ada Lovelace\n"
        "  user_email: ada@example.com\n"
        "  github_user: ada\n",
        encoding="utf-8",
    )
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    assert result.exit_code == 0, result.output
    answers = read_answers(tmp_path / "demo")
    assert answers["user_name"] == "Ada Lovelace"
    assert answers["user_email"] == "ada@example.com"
    assert answers["github_user"] == "ada"


@pytest.mark.parametrize("missing", PERSONAL_FLAGS)
def test_new_requires_each_personal_answer_with_no_input(
    missing: str, tmp_path: pathlib.Path, copier_settings: pathlib.Path
):
    """Test that with no settings file default and no flag for one personal question,
    --no-input names that question and creates nothing.
    """
    copier_settings.write_text("defaults: {}\n", encoding="utf-8")
    given = [
        arg
        for question, args in PERSONAL_FLAGS.items()
        if question != missing
        for arg in args
    ]
    result = CliRunner().invoke(app, ["new", "demo", "--no-input", *given])
    assert result.exit_code != 0, result.output
    assert missing in result.output
    assert not (tmp_path / "demo").exists()


# Flags override the personal defaults


@pytest.mark.parametrize(
    ("flag", "question", "value"),
    [
        ("--user-name", "user_name", "Grace Hopper"),
        ("--user-email", "user_email", "grace@example.com"),
        ("--github-user", "github_user", "grace"),
    ],
)
def test_new_personal_flag_overrides_the_copier_settings_file(
    flag: str, question: str, value: str, tmp_path: pathlib.Path
):
    """Test that each personal flag beats the value in Copier's settings file."""
    result = CliRunner().invoke(app, ["new", "demo", "--no-input", flag, value])
    assert result.exit_code == 0, result.output
    assert read_answers(tmp_path / "demo")[question] == value


# The backend question and its flag


def test_new_records_no_backend_by_default(tmp_path: pathlib.Path):
    """Test that a project gets no backend unless one is asked for."""
    result = CliRunner().invoke(app, ["new", "demo", "--no-input"])
    assert result.exit_code == 0, result.output
    assert read_answers(tmp_path / "demo")["backend"] == "none"


def test_new_records_the_backend_flag(tmp_path: pathlib.Path):
    """Test that an answer passed as a flag reaches the rendered project."""
    result = CliRunner().invoke(
        app, ["new", "demo", "--no-input", "--backend", "python"]
    )
    assert result.exit_code == 0, result.output
    assert read_answers(tmp_path / "demo")["backend"] == "python"


def test_new_rejects_an_unknown_backend(tmp_path: pathlib.Path):
    """Test that an unknown backend exits non-zero, names the valid stacks and creates nothing."""
    result = CliRunner().invoke(app, ["new", "demo", "--no-input", "--backend", "rust"])
    assert result.exit_code != 0, result.output
    assert "none" in result.output
    assert "python" in result.output
    assert not (tmp_path / "demo").exists()


# The minimal preset


def test_new_minimal_preset_renders_no_components(
    tmp_path: pathlib.Path, copier_settings: pathlib.Path
):
    """Test that --preset minimal turns every component off, even when the settings file
    defaults to one.
    """
    copier_settings.write_text(
        copier_settings.read_text(encoding="utf-8") + "  backend: python\n",
        encoding="utf-8",
    )
    result = CliRunner().invoke(
        app, ["new", "demo", "--preset", "minimal", "--no-input"]
    )
    assert result.exit_code == 0, result.output
    assert read_answers(tmp_path / "demo")["backend"] == "none"
    assert not (tmp_path / "demo" / "backend").exists()


def test_new_flag_overrides_the_preset(tmp_path: pathlib.Path):
    """Test that a flag beats the preset's answer for the same question."""
    result = CliRunner().invoke(
        app, ["new", "demo", "--preset", "minimal", "--backend", "python", "--no-input"]
    )
    assert result.exit_code == 0, result.output
    assert read_answers(tmp_path / "demo")["backend"] == "python"


# An unknown preset


def test_new_rejects_an_unknown_preset_and_lists_the_presets(tmp_path: pathlib.Path):
    """Test that an unknown preset exits non-zero, lists the presets and creates nothing."""
    result = CliRunner().invoke(app, ["new", "demo", "--preset", "huge", "--no-input"])
    assert result.exit_code != 0, result.output
    assert "minimal" in result.output
    assert not (tmp_path / "demo").exists()


# Prompting without --no-input
#
# CliRunner has no terminal, so Copier prints the first question it asks and then fails with
# InteractiveSessionError. That's enough to see which question it asked.


def test_new_prompts_for_unanswered_questions_without_no_input():
    """Test that without --no-input, sow new asks the first unanswered question."""
    result = CliRunner().invoke(app, ["new", "demo"])
    assert USER_NAME_HELP in result.output


def test_new_does_not_prompt_for_answers_given_as_flags():
    """Test that questions answered by flags are skipped and the next unanswered one is asked."""
    given = [arg for args in PERSONAL_FLAGS.values() for arg in args]
    result = CliRunner().invoke(app, ["new", "demo", *given])
    assert USER_NAME_HELP not in result.output
    assert BACKEND_HELP in result.output


def test_new_without_a_terminal_exits_with_a_message(tmp_path: pathlib.Path):
    """Test that when sow new can't prompt, it exits with a message pointing at --no-input
    instead of a traceback, and creates nothing.
    """
    result = CliRunner().invoke(app, ["new", "demo"])
    assert result.exit_code == 1, result.output
    assert "--no-input" in result.output
    assert not (tmp_path / "demo").exists()
