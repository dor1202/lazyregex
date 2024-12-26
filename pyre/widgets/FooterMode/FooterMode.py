from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Label
from pyre.GlobalState import GlobalState


class FooterMode(Widget):
    DEFAULT_CSS = """
    FooterMode {
        layout: horizontal;
    }
    FooterMode Label {
        background: green;
        margin: 0 1;
    }
    """

    def __init__(self):
        super().__init__(id="FooterMode")

    def compose(self) -> ComposeResult:
        yield Label(GlobalState().regex_method)
