import enum
import os
import subprocess
from importlib.metadata import version
from typing import Annotated

import copier
import typer

app = typer.Typer()


class Preset(enum.StrEnum):
    """A name set of answers that 'sow new' fills in."""

    MINIMAL = "minimal"


PRESETS: dict[Preset, dict[str, str]] = {Preset.MINIMAL: {"backend": "none"}}


@app.command()
def new(
    name: str,
    no_input: Annotated[bool, typer.Option("--no-input")] = False,
    user_name: Annotated[str | None, typer.Option()] = None,
    user_email: Annotated[str | None, typer.Option()] = None,
    github_user: Annotated[str | None, typer.Option()] = None,
    backend: Annotated[str | None, typer.Option()] = None,
    preset: Annotated[Preset | None, typer.Option()] = None,
):
    """Generates a new project with a template."""
    template_src = os.environ.get(
        "SOW_TEMPLATE_SRC", "https://github.com/MMaahir/sow.git"
    )
    template_ref = os.environ.get("SOW_TEMPLATE_REF", f"v{version('sow')}")
    flags = {
        "user_name": user_name,
        "user_email": user_email,
        "github_user": github_user,
        "backend": backend,
    }
    given_flags = {k: v for k, v in flags.items() if v is not None}
    try:
        copier.run_copy(
            src_path=template_src,
            dst_path=name,
            data={
                **(PRESETS[preset] if preset else {}),
                **given_flags,
                "project_name": name,
            },
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
