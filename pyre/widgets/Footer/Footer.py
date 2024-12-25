from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Footer


class CustomFooter(Widget):
    CSS_PATH = "Footer.tcss"

    def compose(self) -> ComposeResult:
        yield Footer()
