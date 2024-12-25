from textual.screen import Screen
from textual.app import ComposeResult
from textual.widgets import Label

from ...widgets.RegexOptions.RegexOptions import RegexOptions


class RegexOptionsScreen(Screen):
    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield Label("Regex Options")
        yield RegexOptions()
