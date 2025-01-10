from textual.app import ComposeResult
from textual.screen import Screen

from ...widgets.widgets.RegexOptions.RegexOptions import RegexOptions


class RegexOptionsScreen(Screen):
    CSS_PATH = "RegexOptions.tcss"

    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield RegexOptions()
