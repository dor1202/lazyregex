from textual.app import ComposeResult
from textual.screen import Screen

from ...widgets.widgets.RegexModes.RegexModes import RegexModes


class RegexModesScreen(Screen):
    CSS_PATH = "RegexModes.tcss"

    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield RegexModes()
