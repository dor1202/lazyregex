from textual.screen import Screen
from textual.app import ComposeResult
from textual.containers import Grid

from ...widgets.CmdInput.CmdInput import CmdInput
from ...widgets.CustomHeader.CustomHeader import CustomHeader
from ...widgets.Footer.Footer import CustomFooter
from ...widgets.GroupsArea.GroupsArea import GroupsArea
from ...widgets.InputArea.InputArea import InputArea
from ...widgets.PatternInput.PatternInput import PatternInput
from ...widgets.FooterOptions.FooterOptions import FooterOptions
from ...widgets.FooterMode.FooterMode import FooterMode

class HomeScreen(Screen):
    CSS_PATH = "Home.tcss"

    BINDINGS = [
        (":", "open_cmd"),
    ]

    def action_open_cmd(self):
        self.query_one(CmdInput).display = "block"
        self.query_one(CmdInput).disabled = False
        self.query_one(CmdInput).focus()

    def compose(self) -> ComposeResult:
        yield CustomHeader()
        yield CmdInput()
        yield PatternInput()
        yield Grid(
            InputArea(),
            GroupsArea()
            , classes="hitsArea"
        )
        yield Grid(
            FooterOptions(id="FooterOptions"),
            FooterMode(id="FooterMode")
            , classes="FooterArea"
        )
