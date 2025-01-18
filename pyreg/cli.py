import typer

from .pyreg import PyregApp

app = typer.Typer()


@app.command("pyreg")
def pyre_cli() -> None:
    PyregApp().run()

# Debug code:
# Terminal 1: textual console
# Terminal 1: textual run --dev pyreg.cli:app

# Publish release:
# poetry run bump-my-version patch
# git push --tags