from textual.screen import Screen
from textual.app import ComposeResult
from textual.containers import Horizontal

from ...widgets.CmdInput.CmdInput import CmdInput
from ...widgets.Footer.Footer import CustomFooter
from ...widgets.GeneralData.GeneralData import GeneralData
from ...widgets.InputArea.InputArea import InputArea
from ...widgets.Logo.Logo import Logo
from ...widgets.PatternInput.PatternInput import PatternInput


class HomeScreen(Screen):
    BINDINGS = [
        (":", "open_cmd"),
    ]

    def action_open_cmd(self):
        self.query_one(CmdInput).visible = True
        self.query_one(CmdInput).disabled = False
        self.query_one(CmdInput).focus()

    def compose(self) -> ComposeResult:
        with Horizontal(id="Top"):
            yield GeneralData()
            yield Logo()
        yield CmdInput()
        yield PatternInput()
        yield InputArea()
        yield CustomFooter()
