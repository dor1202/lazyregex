import typer

from .screens.home import LayoutApp
from .pyre import PyreApp

app = typer.Typer()


@app.command("pyre")
def pyre_cli(
) -> None:
    app = LayoutApp(
    )
    app.run()
