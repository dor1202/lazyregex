import platform

from textual.app import ComposeResult
from textual.containers import Container
from textual.widget import Widget
from textual.widgets import Label

from .... import __version__


class GeneralData(Widget):
    DEFAULT_CSS = """
    .vertical-layout {
        layout: vertical;
        height: 4;
        width: auto;
    }
    
    .horizontal-layout {
        layout: horizontal;
        height: auto;
        width: auto;
    }
    
    Label {
        width: auto;
        padding: 0 1;
    }
    
    .orangeColor {
        color: orange;
    }
    """

    def __init__(self):
        super().__init__(id="Header")

    def compose(self) -> ComposeResult:
        yield Container(
            Container(
                Label("Application version:", classes="orangeColor"),
                Label(__version__),
                classes="horizontal-layout",
            ),
            Container(
                Label("Python version:", classes="orangeColor"),
                Label(platform.python_version()),
                classes="horizontal-layout",
            ),
            classes="vertical-layout",
        )
