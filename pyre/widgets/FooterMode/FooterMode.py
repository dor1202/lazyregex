from rich.console import RenderableType
from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Label
from textual.reactive import reactive
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

    mode = reactive("match", recompose=True)

    def compose(self) -> ComposeResult:
        yield Label(self.mode)
