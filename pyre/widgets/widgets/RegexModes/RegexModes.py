from textual.app import ComposeResult
from textual.reactive import reactive
from textual.widget import Widget
from textual.widgets import Label, RadioButton, RadioSet

from ....logic.GlobalState import GlobalState


class RegexModes(Widget):
    BORDER_TITLE = "Regex Modes"

    OPTIONS = ["match", "substitution"]

    def compose(self) -> ComposeResult:
        with RadioSet():
            for option in self.OPTIONS:
                yield RadioButton(option, value=(option == GlobalState().regex_method))

    def on_radio_set_changed(self, event: RadioSet.Changed) -> None:
        GlobalState().regex_method = self.OPTIONS[event.index]

        from ..FooterMode.FooterMode import FooterMode


        main_screen = self.app.screen_stack[-2]
        main_screen.regex_method = self.OPTIONS[event.index]
        main_screen.query_one(FooterMode).mode = self.OPTIONS[event.index]
