import re

from textual.widgets import Input

from ....highlighters.pattern_highlight import PatternHighlighter
from ....logic.Debouncer import Debouncer
from ....logic.GlobalState import GlobalState
from ....logic.RegexLogic import RegexLogic
from ....widgets.widgets.GroupsArea.GroupsArea import GroupsArea
from ..ColoredInputArea.ColoredInputArea import ColoredInputArea


class PatternInput(Input):
    DEFAULT_CSS = """
    PatternInput:disabled {
        opacity: 100% !important;
    }
    """

    BORDER_TITLE = "Pattern"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(
            id="PatternInput", disabled=True, highlighter=PatternHighlighter()
        )
        self.debouncer = Debouncer(0.5)

    def action_drop_focus_input(self):
        self.disabled = True

        from ....widgets.widgets.Help.Help import Help
        GlobalState().help_ui = [
            ("<Shift + :>", "Commands Input"),
        ]
        self.app.query_one(Help).help_labels = GlobalState().help_ui

    async def on_input_changed(self):
        await self.debouncer.debounce(self.process_input)

    def process_input(self):
        will_regex_crush = re.search(r"\\(?!(w|s|d))", self.value, re.IGNORECASE)
        if will_regex_crush:
            return

        RegexLogic().update_pattern(self.value)
        self.app.query_one(GroupsArea).groups = GlobalState().groups
        self.app.query_one(ColoredInputArea).process_input()
        # Renders the highlights
        self.app.query_one(ColoredInputArea).disabled = False
        self.app.query_one(ColoredInputArea).disabled = True

