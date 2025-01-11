import typer

from .tmp_pyre import PyreApp

app = typer.Typer()


@app.command("pyre")
def pyre_cli() -> None:
    PyreApp().run()

# Debug code:
# Terminal 1: textual console
# Terminal 1: textual run --dev pyre_tui.cli:app

# Publish release:
# poetry run bump-my-version patch
# git push --tags