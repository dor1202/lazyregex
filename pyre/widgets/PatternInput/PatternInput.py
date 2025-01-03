from textual.widgets import Input

from ..ColoredInputArea.ColoredInputArea import ColoredInputArea
from ..GroupsArea.GroupsArea import GroupsArea
from ...Logic.Debouncer import Debouncer
from ...Logic.RegexLogic import RegexLogic
from ...highlighters.pattern_highlight import PatternHighlighter


class PatternInput(Input):
    DEFAULT_CSS = """
    .regex .group {
        color: red;
    }
    """

    BORDER_TITLE = "Pattern"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="PatternInput", disabled=True, highlighter=PatternHighlighter())
        self.debouncer = Debouncer(0.5)

    def action_drop_focus_input(self):
        self.disabled = True

    async def on_input_changed(self):
        await self.debouncer.debounce(self.process_input)

    def process_input(self):
        RegexLogic().update_pattern(self.value)
        self.app.query_one(GroupsArea).groups = RegexLogic().groups
        self.app.query_one(ColoredInputArea).process_input()
