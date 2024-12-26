import typer

from .tmp_pyre import PyreApp

# Init the GlobalState
from .GlobalState import GlobalState # noqa

app = typer.Typer()


@app.command("pyre")
def pyre_cli() -> None:
    PyreApp().run()
