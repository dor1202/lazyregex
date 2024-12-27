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
        width: 90%;
    }
    FooterOptions Label {
        background: orange;
        width: 10;
    }
    """

    options = reactive(["aaa", "bbb"], recompose=True)

    def compose(self) -> ComposeResult:
        yield Container(
            *[Label(option) for option in self.options],
            *[Label(option) for option in self.options],
        )
