from textual.app import ComposeResult
from textual.screen import Screen

from ...widgets.widgets.HelpData.HelpData import HelpData


class HelpScreen(Screen):
    CSS_PATH = "HelpScreen.tcss"

    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield HelpData()
