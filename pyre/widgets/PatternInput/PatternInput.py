from textual.widgets import Input

from ..GroupsArea.GroupsArea import GroupsArea
from ...Logic.RegexLogic import RegexLogic


class PatternInput(Input):
    BORDER_TITLE = "Pattern"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="PatternInput", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True

    def on_input_changed(self, new_value: Input.Changed):
        RegexLogic().update_pattern(new_value.value)
        self.app.query_one(GroupsArea).groups = RegexLogic().groups
