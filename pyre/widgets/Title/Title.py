from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import Rule, Label


class Title(Widget):
    DEFAULT_CSS = """
    Title {
        height: 3;
        layout: vertical;
    }
    
    Title Rule {
        color: rgb(84, 84, 115);
    }
    
    Title Label {
        width: 100%;
        text-align: center;
    }
    """

    def __init__(self):
        super().__init__(id="Title")

    def compose(self) -> ComposeResult:
        yield Label("<FILL>")
        yield Rule()
