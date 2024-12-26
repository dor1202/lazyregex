from textual.widgets import Input


class PatternInput(Input):
    BORDER_TITLE = "Pattern"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="PatternInput", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True
