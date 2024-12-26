from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Label


class CustomFooter(Widget):
    DEFAULT_CSS = """
    CustomFooter {
        height: 1;
        layout: horizontal;
    }
    CustomFooter Label {
        background: orange;
        margin: 0 1;
    }
    """

    def __init__(self):
        super().__init__(id="CustomFooter")

    def compose(self) -> ComposeResult:
        yield Label("aaa")
        yield Label("bbb")
