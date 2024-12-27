from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import Label
from textual.reactive import reactive
from textual.containers import Container

from pyre.GlobalState import GlobalState


class FooterOptions(Widget):
    DEFAULT_CSS = """
    FooterOptions {
        layout: horizontal;
    }
    FooterOptions Container {
        layout: horizontal;
    }
    .optionLabel {
        background: rgb(255, 140, 0);
        width: auto;
        margin: 0 1;
    }
    """

    options = reactive([], recompose=True)

    def compose(self) -> ComposeResult:
        yield Label("Options:")
        yield Container(
            *[Label(option, classes="optionLabel") for option in self.options],
        )
