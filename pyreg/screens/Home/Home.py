from textual.app import ComposeResult
from textual.containers import Grid
from textual.screen import Screen
from textual.reactive import reactive

from ...logic.GlobalState import GlobalState
from ...widgets.inputs.SubstitutionInput.SubstitutionInput import SubstitutionInput
from ...widgets.inputs.CmdInput.CmdInput import CmdInput
from ...widgets.inputs.ColoredInputArea.ColoredInputArea import ColoredInputArea
from ...widgets.inputs.PatternInput.PatternInput import PatternInput
from ...widgets.widgets.CustomHeader.CustomHeader import CustomHeader
from ...widgets.widgets.FooterMode.FooterMode import FooterMode
from ...widgets.widgets.FooterOptions.FooterOptions import FooterOptions
from ...widgets.widgets.GroupsArea.GroupsArea import GroupsArea
from ...widgets.inputs.SubstitutionInputArea.SubstitutionInputArea import SubstitutionInputArea

class HomeScreen(Screen):
    CSS_PATH = "Home.tcss"

    BINDINGS = [
        (":", "open_cmd"),
    ]
    
    regex_method = reactive(GlobalState().regex_method, recompose=True)

    def action_open_cmd(self):
        self.query_one(CmdInput).display = "block"
        self.query_one(CmdInput).disabled = False
        self.query_one(CmdInput).focus()

    def compose(self) -> ComposeResult:
        yield CustomHeader()
        yield CmdInput()
        yield PatternInput()
        if self.regex_method == "match":
            yield Grid(ColoredInputArea(), GroupsArea(), classes="hitsArea")
            yield Grid(
                FooterOptions(id="FooterOptions"),
                FooterMode(id="FooterMode", mode=self.regex_method),
                classes="FooterArea",
            )
        elif self.regex_method == "substitution":
            yield SubstitutionInput()
            yield Grid(ColoredInputArea(), GroupsArea(), classes="hitsArea")
            yield SubstitutionInputArea()
            yield Grid(
                FooterOptions(id="FooterOptions"),
                FooterMode(id="FooterMode", mode=self.regex_method),
                classes="FooterArea",
            )
        else:
            pass
