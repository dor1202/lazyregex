from textual.widgets import TextArea


class InputArea(TextArea):
    CSS_PATH = "InputArea.tcss"

    BORDER_TITLE = "Test String"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="InputArea", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True