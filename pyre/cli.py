import typer

from .pyre import PyreApp

app = typer.Typer()


@app.command("pyre")
def pyre_cli(
) -> None:
    app: PyreApp[int] = PyreApp(
        "", initial_mode=None, initial_pattern=None
    )
    app.run()
    print(app.pattern)
