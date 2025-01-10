from textual.app import ComposeResult
from textual.containers import Container
from textual.widget import Widget
from textual.widgets import Label
from textual.reactive import reactive

from ....logic.GlobalState import GlobalState


class Help(Widget):
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
    
    .HelpColor {
        color: cyan;
    }
    """

    help_labels = reactive(GlobalState().help_ui, recompose=True)

    def __init__(self):
        super().__init__(id="Help")

    def compose(self) -> ComposeResult:
        containers = [
            Container(
                Label(command, classes="HelpColor"),
                Label(explanation),
                classes="horizontal-layout",
            )
            for command, explanation in self.help_labels
        ]
        yield Container(
            *containers,
            classes="vertical-layout",
        )
