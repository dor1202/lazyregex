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
        # Will cause in infinite loop
        # self.text = self.create_highlighted_output(RegexLogic().raw_groups)

    def create_highlighted_output(self, groups_matches) -> str:
        UNDERLINE = "\033[4m"
        RESET_UNDERLINE = "\033[24m"

        output = ""
        for character in range(len(self.text)):
            # if character in first_starts and character not in first_ends:
            #     output += UNDERLINE
            # if character in starts and character not in ends:
            #     output += Fore.RED
            # if character in ends and character not in starts:
            #     output += Fore.RESET
            # if character in first_ends and character not in first_starts:
            #     output += RESET_UNDERLINE
            output += self.text[character]

        #     output += Fore.RESET
        # if input_length in ends:
        # if input_length in first_ends:
        #     output += RESET_UNDERLINE

        return output
