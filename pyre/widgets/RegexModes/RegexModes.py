from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import RadioButton, RadioSet, Label
from textual.reactive import reactive

from ...GlobalState import GlobalState


class RegexModes(Widget):
    OPTIONS = ["match", "substitution", "search"]

    def compose(self) -> ComposeResult:
        with RadioSet():
            for option in self.OPTIONS:
                yield RadioButton(option, value=(option == GlobalState().regex_method))

    def on_radio_set_changed(self, event: RadioSet.Changed) -> None:
        GlobalState().regex_method= self.OPTIONS[event.index]

        # from ..FooterMode.FooterMode import FooterMode
        # main_screen = self.app.screen_stack[-2]
        # main_screen.query_one(FooterMode).method = self.OPTIONS[event.index]
