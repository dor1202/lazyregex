from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Label
from pyre.GlobalState import GlobalState


class FooterOptions(Widget):
    DEFAULT_CSS = """
    FooterOptions {
        layout: horizontal;
        width: 95%;
    }
    FooterOptions Label {
        background: orange;
        margin: 0 1;
    }
    """

    def __init__(self):
        super().__init__(id="FooterOptions")

    def compose(self) -> ComposeResult:
        for option in GlobalState().regex_options:
            yield Label(option)
