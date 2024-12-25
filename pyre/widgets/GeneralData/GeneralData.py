from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import Label
from textual.containers import Grid
import platform


class GeneralData(Widget):
    CSS_PATH = "GeneralData.tcss"

    def __init__(self):
        super().__init__(id="Header")

    def compose(self) -> ComposeResult:
        yield Grid(
            Label("Python version:", classes="GridDesc"),
            Label(platform.python_version()),
        )
