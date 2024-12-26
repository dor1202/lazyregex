from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import RadioButton, RadioSet


class RegexModes(Widget):
    def compose(self) -> ComposeResult:
        with RadioSet():
            yield RadioButton("match")
            yield RadioButton("substitution")
            yield RadioButton("search")
