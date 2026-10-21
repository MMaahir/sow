import os
import subprocess
from importlib.metadata import version
from typing import Annotated

import copier
import typer

app = typer.Typer()


@app.command()
def new(name: str, no_input: Annotated[bool, typer.Option("--no-input")] = False):
    """Generates a new project with a template."""
    sow_template = os.environ.get(
        "SOW_TEMPLATE_SRC", "https://github.com/MMaahir/sow.git"
    )
    sow_ref = os.environ.get("SOW_TEMPLATE_REF", f"v{version('sow')}")
    copier.run_copy(
        src_path=sow_template,
        dst_path=name,
        data={"project_name": name},
        defaults=no_input,
        vcs_ref=sow_ref,
    )
    subprocess.run(
        ["git", "init", "-b", "main"],
        check=True,
        cwd=name,
    )
    subprocess.run(
        ["git", "add", "-A"],
        check=True,
        cwd=name,
    )
    subprocess.run(
        ["git", "commit", "-m", "chore: initial commit from sow"],
        check=True,
        cwd=name,
    )


@app.callback()
def callback():
    """Set your projects up faster with sow."""
