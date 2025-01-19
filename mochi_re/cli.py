import typer

from .mochi import MochiApp

app = typer.Typer()


@app.command("mochi")
def mochi_cli() -> None:
    MochiApp().run()

# Debug code:
# Terminal 1: textual console
# Terminal 1: textual run --dev mochi_re.cli:app

# Publish release:
# poetry run bump-my-version patch
# git push --tags