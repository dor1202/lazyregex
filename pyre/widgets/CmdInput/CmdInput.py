from textual.widgets import Input
from textual.suggester import SuggestFromList

class CmdInput(Input):
    BINDINGS = [
        ("escape", "close_input"),
    ]

    def __init__(self):
        super().__init__(
            id="CmdInput",
            disabled=True,
            suggester=SuggestFromList(
                ["q", "q!", "quit", "p", "pattern", "i", "input", "m", "mode", "o", "options", "h", "help"]
            )
        )

    def action_close_input(self):
        self.display = "none"
        self.disabled = True
        self.value = ""

    def focus_pattern(self):
        self.action_close_input()

        from ...screens.Home.Home import PatternInput
        self.app.query_one(PatternInput).disabled = False
        self.app.query_one(PatternInput).focus()

    def focus_input(self):
        self.action_close_input()

        from ...widgets.InputArea.InputArea import InputArea
        self.app.query_one(InputArea).disabled = False
        self.app.query_one(InputArea).focus()

    def open_modes(self):
        self.action_close_input()
        self.app.push_screen("modes")

    def open_options(self):
        self.action_close_input()
        self.app.push_screen("options")

    def open_help(self):
        self.action_close_input()
        self.app.push_screen("help")

    def action_submit(self) -> None:
        commands = {
            "q": lambda: self.app.exit(),
            "q!": lambda: self.app.exit(),
            "quit": lambda: self.app.exit(),
            "p": self.focus_pattern,
            "pattern": self.focus_pattern,
            "i": self.focus_input,
            "input": self.focus_input,
            "m": self.open_modes,
            "mode": self.open_modes,
            "o": self.open_options,
            "options": self.open_options,
            "h": self.open_help,
            "help": self.open_help,
        }
        com = commands.get(self.value, self.action_close_input)
        com()
