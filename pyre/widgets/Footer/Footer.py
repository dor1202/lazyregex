from textual.app import ComposeResult
from textual.widget import Widget

from ..FooterMode.FooterMode import FooterMode
from ..FooterOptions.FooterOptions import FooterOptions


class CustomFooter(Widget):
    DEFAULT_CSS = """
    CustomFooter {
        height: 1;
        layout: horizontal;
    }
    """

    def __init__(self):
        super().__init__(id="CustomFooter")

    def compose(self) -> ComposeResult:
        yield FooterOptions()
        yield FooterMode()
