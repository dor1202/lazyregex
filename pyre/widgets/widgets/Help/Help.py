from textual.app import ComposeResult
from textual.containers import Container
from textual.widget import Widget
from textual.widgets import Label


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

    def __init__(self):
        super().__init__(id="Help")

    def compose(self) -> ComposeResult:
        yield Container(
            Container(
                Label("<Shift + :>", classes="HelpColor"),
                Label("Commands Input"),
                classes="horizontal-layout",
            ),
            Container(
                Label("<Esc>", classes="HelpColor"),
                Label("Drop Focus"),
                classes="horizontal-layout",
            ),
            classes="vertical-layout",
        )
