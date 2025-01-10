import typer

from .tmp_pyre import PyreApp

app = typer.Typer()


@app.command("pyre")
def pyre_cli() -> None:
    PyreApp().run()
