import typer

from .moch import MochApp

app = typer.Typer()


@app.command("moch")
def moch_cli() -> None:
    MochApp().run()

# Debug code:
# Terminal 1: textual console
# Terminal 1: textual run --dev moch.cli:app

# Publish release:
# poetry run bump-my-version patch
# git push --tags