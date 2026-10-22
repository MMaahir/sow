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
    template_src = os.environ.get(
        "SOW_TEMPLATE_SRC", "https://github.com/MMaahir/sow.git"
    )
    template_ref = os.environ.get("SOW_TEMPLATE_REF", f"v{version('sow')}")
    try:
        copier.run_copy(
            src_path=template_src,
            dst_path=name,
            data={"project_name": name},
            defaults=no_input,
            vcs_ref=template_ref,
        )
    except ValueError as error:
        typer.echo(error, err=True)
        raise typer.Exit(code=1)

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
