from pathlib import Path

import pytest
from prompt_toolkit.application import create_app_session

GIT_CONFIG_FOR_TESTS: str = """\
[user]
    name = Test User
    email = test@example.com

[init]
    defaultBranch = master
"""

COPIER_SETTINGS_FOR_TESTS: str = """\
defaults:
  user_name: Test User
  user_email: test@example.com
  github_user: test-user
"""

REPO_ROOT = Path(__file__).resolve().parents[1]


@pytest.fixture
def copier_settings(tmp_path: Path) -> Path:
    """The Copier settings file sow reads in tests. Overwrite it to change the personal defaults."""
    return tmp_path / "copier-settings.yml"


@pytest.fixture(autouse=True)
def isolated_env(
    tmp_path: Path, copier_settings: Path, monkeypatch: pytest.MonkeyPatch
):
    """Isolate git and Copier and point sow at this checkout so tests don't depend on this machine."""
    p = tmp_path / "gitconfig"
    p.write_text(GIT_CONFIG_FOR_TESTS, encoding="utf-8")
    monkeypatch.setenv("GIT_CONFIG_GLOBAL", str(p))
    monkeypatch.setenv("GIT_CONFIG_NOSYSTEM", "1")
    copier_settings.write_text(COPIER_SETTINGS_FOR_TESTS, encoding="utf-8")
    monkeypatch.setenv("COPIER_SETTINGS_PATH", str(copier_settings))
    monkeypatch.setenv("SOW_TEMPLATE_SRC", str(REPO_ROOT))
    monkeypatch.setenv("SOW_TEMPLATE_REF", "HEAD")
    monkeypatch.chdir(tmp_path)


@pytest.fixture(autouse=True)
def fresh_prompt_session():
    """Give each test its own prompt_toolkit session.

    prompt_toolkit keeps the first stdin it sees for the whole process, and CliRunner closes that
    stream after each call, so the next test that prompts would read from a closed file.
    """
    with create_app_session():
        yield
