from textual.suggester import SuggestFromList
from textual.widgets import Input

from ....logic.GlobalState import GlobalState


class CmdInput(Input):
    BINDINGS = [
        ("escape", "close_input"),
    ]

    def __init__(self):
        super().__init__(
            id="CmdInput",
            disabled=True,
            suggester=SuggestFromList(
                [
                    "q",
                    "q!",
                    "quit",
                    "p",
                    "pattern",
                    "i",
                    "input",
                    "m",
                    "mode",
                    "o",
                    "options",
                    "h",
                    "help",
                    "s",
                    "substitution",
                ]
            ),
        )

    def action_close_input(self):
        self.display = "none"
        self.disabled = True
        self.value = ""

    def focus_pattern(self):
        self.action_close_input()

        from ....screens.Home.Home import PatternInput

        self.app.query_one(PatternInput).disabled = False
        self.app.query_one(PatternInput).focus()

        from ....widgets.widgets.Help.Help import Help
        GlobalState().help_ui = [
            ("<Esc>", "Drop Focus"),
        ]
        self.app.query_one(Help).help_labels = GlobalState().help_ui
    
    def focus_substitution(self):
        self.action_close_input()

        if GlobalState().regex_method == "substitution":
            from ....widgets.inputs.SubstitutionInput.SubstitutionInput import SubstitutionInput
            self.app.query_one(SubstitutionInput).disabled = False
            self.app.query_one(SubstitutionInput).focus()

            from ....widgets.widgets.Help.Help import Help
            GlobalState().help_ui = [
                ("<Esc>", "Drop Focus"),
            ]
            self.app.query_one(Help).help_labels = GlobalState().help_ui

    def focus_input(self):
        self.action_close_input()

        from ....widgets.inputs.ColoredInputArea.ColoredInputArea import (
            ColoredInputArea,
        )

        self.app.query_one(ColoredInputArea).disabled = False
        self.app.query_one(ColoredInputArea).focus()

        from ....widgets.widgets.Help.Help import Help
        GlobalState().help_ui = [
            ("<Esc>", "Drop Focus"),
        ]
        self.app.query_one(Help).help_labels = GlobalState().help_ui

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
            "s": self.focus_substitution,
            "substitution": self.focus_substitution,
        }
        com = commands.get(self.value, self.action_close_input)
        com()
