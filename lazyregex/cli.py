import typer

from .lazyregex import LazyRegexApp

app = typer.Typer()


@app.command("lazyregex")
def lazyregex_cli() -> None:
    LazyRegexApp().run()

# Debug code:
# Terminal 1: textual console
# Terminal 1: textual run --dev lazyregex.cli:app

# Publish release:
# poetry run bump-my-version patch
# git push --tags