from textual.screen import Screen
from textual.app import ComposeResult

from ...widgets.CmdInput.CmdInput import CmdInput
from ...widgets.CustomHeader.CustomHeader import CustomHeader
from ...widgets.Footer.Footer import CustomFooter
from ...widgets.InputArea.InputArea import InputArea
from ...widgets.PatternInput.PatternInput import PatternInput


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
        yield InputArea()
        yield CustomFooter()
