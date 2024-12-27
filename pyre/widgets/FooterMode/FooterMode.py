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

    .modeLabel {
        margin: 0 2;
    }
    
    .modeValue {
        background: green;
        margin: 0;
    }
    """

    mode = reactive("match", recompose=True)

    def compose(self) -> ComposeResult:
        yield Label(f"Mode: ", classes="modeLabel")
        yield Label(self.mode, classes="modeValue")
