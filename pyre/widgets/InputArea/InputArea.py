from textual.widgets import TextArea

from ..GroupsArea.GroupsArea import GroupsArea
from ...Logic.RegexLogic import RegexLogic


class InputArea(TextArea):
    DEFAULT_CSS = """
    InputArea {
        width: 50%;
    }
    """

    BORDER_TITLE = "Test String"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="InputArea", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True

    def on_text_area_changed(self):
        RegexLogic().update_text(self.text)
        self.app.query_one(GroupsArea).groups = RegexLogic().groups
