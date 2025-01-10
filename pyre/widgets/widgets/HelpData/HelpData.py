from textual.app import ComposeResult
from textual.containers import Container
from textual.widget import Widget
from textual.widgets import Label


class HelpData(Widget):
    BORDER_TITLE = "Help"

    def __init__(self):
        super().__init__(id="Help")

    def compose(self) -> ComposeResult:
        yield Container(
            Container(
                Label("q / q! / quit", classes="HelpColor"),
                Label("Exit the application"),
                classes="horizontal-layout",
            ),
            Container(
                Label("p / pattern", classes="HelpColor"),
                Label("Focus the pattern input"),
                classes="horizontal-layout",
            ),
            Container(
                Label("i / input", classes="HelpColor"),
                Label("Focus the input area"),
                classes="horizontal-layout",
            ),
            Container(
                Label("m / mode", classes="HelpColor"),
                Label("Open the modes screen"),
                classes="horizontal-layout",
            ),
            Container(
                Label("o / options", classes="HelpColor"),
                Label("Open the options screen"),
                classes="horizontal-layout",
            ),
            classes="vertical-layout",
        )
