from pathlib import Path

import pytest

GIT_CONFIG_FOR_TESTS: str = """\
[user]
    name = Test User
    email = test@example.com
"""

REPO_ROOT = Path(__file__).resolve().parents[1]


@pytest.fixture(autouse=True)
def isolated_env(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    """Isolate git and point sow at this checkout so tests don't depend on this machine."""
    p = tmp_path / "gitconfig"
    p.write_text(GIT_CONFIG_FOR_TESTS, encoding="utf-8")
    monkeypatch.setenv("GIT_CONFIG_GLOBAL", str(p))
    monkeypatch.setenv("GIT_CONFIG_NOSYSTEM", "1")
    monkeypatch.setenv("SOW_TEMPLATE_SRC", str(REPO_ROOT))
    monkeypatch.setenv("SOW_TEMPLATE_REF", "HEAD")
    monkeypatch.chdir(tmp_path)
