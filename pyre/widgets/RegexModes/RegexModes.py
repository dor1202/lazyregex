from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import RadioButton, RadioSet, Label

from ...GlobalState import GlobalState


class RegexModes(Widget):
    def compose(self) -> ComposeResult:
        with RadioSet():
            yield RadioButton("match", value=True)
            yield RadioButton("substitution")
            yield RadioButton("search")

    def on_radio_set_changed(self, event: RadioSet.Changed) -> None:
        GlobalState().regex_method = event.index
