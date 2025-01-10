from textual.widgets import Input

from ....widgets.inputs.SubstitutionInputArea.SubstitutionInputArea import SubstitutionInputArea
from ....logic.Debouncer import Debouncer
from ....logic.GlobalState import GlobalState
from ....logic.RegexLogic import RegexLogic


class SubstitutionInput(Input):
    DEFAULT_CSS = """
    SubstitutionInput:disabled {
        opacity: 100% !important;
    }
    """

    BORDER_TITLE = "Substitution Input"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="SubstitutionInput", disabled=True)
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
        RegexLogic().update_substitution_input(self.value)
        self.app.query_one(SubstitutionInputArea).output_text = GlobalState().substitution_output
