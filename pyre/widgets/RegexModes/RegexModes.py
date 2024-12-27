from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import RadioButton, RadioSet, Label
from textual.reactive import reactive

from ...GlobalState import GlobalState


class RegexModes(Widget):
    BORDER_TITLE = "Regex Modes"

    OPTIONS = ["match", "substitution", "search"]

    def compose(self) -> ComposeResult:
        with RadioSet():
            for option in self.OPTIONS:
                yield RadioButton(option, value=(option == GlobalState().regex_method))

    def on_radio_set_changed(self, event: RadioSet.Changed) -> None:
        from ..FooterMode.FooterMode import FooterMode
        main_screen = self.app.screen_stack[-2]
        main_screen.query_one(FooterMode).mode = self.OPTIONS[event.index]
