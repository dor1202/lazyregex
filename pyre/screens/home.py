import platform

from textual import events
from textual.app import App, ComposeResult
from textual.screen import Screen
from textual.widgets import Footer, Static, Input, Label, TextArea, RadioButton, SelectionList, RadioSet
from textual.containers import Horizontal, Vertical, Grid
from textual.widget import Widget

class Logo(Static):
    DEFAULT_CSS = """
    Logo {
        width: 30%;
        height: 5;
        color: orange;
        align-horizontal: right;
    }
    """

    def __init__(self):
        super().__init__(
            renderable="""
            .----..-.  .-..----. .----.
            | {}  }\ \/ / | {}  }| {__/ 
            | .--'  }  {  | .-. \| {__ 
            `-'     `--'  `-' `-'`----'
            """,
            id="Logo"
        )

class Header(Widget):
    DEFAULT_CSS = """
    Header {
        width: 80%;
        height: 5;
    }
    
    Grid {
        grid-size: 2 4;
    }
    
    Label {
        content-align: center middle;
        width: 100%;
        height: 100%;
    }
    
    .GridDesc {
        color: orange;
    }
    """

    def compose(self) -> ComposeResult:
        yield Grid(
            Label("Python version:", classes="GridDesc"),
            Label(platform.python_version()),
        )


class CmdInput(Input):
    BINDINGS = [
        ("escape", "close_input"),
    ]

    DEFAULT_CSS = """
    CmdInput {
        width: 100%;
        visibility: hidden;
    }
    """

    def __init__(self):
        super().__init__(id="CmdInput")

    def action_close_input(self):
        self.visible = False

    def action_submit(self) -> None:
        commands = {
            "q": lambda: self.app.exit(),
            "q!": lambda: self.app.exit()
        }
        com = commands.get(self.value, self.action_close_input)
        com()



class PatternInput(Input):
    DEFAULT_CSS = """
    PatternInput {
        width: 100%;
    }
    """


class InputArea(TextArea):
    DEFAULT_CSS = """
    InputArea {
        width: 100%;
    }
    """


class RegexOptions(SelectionList):
    DEFAULT_CSS = """
    RegexOptions {
        visibility: hidden;
        padding: 1;
        border: solid $accent;
        width: 80%;
        height: 80%;
    }
    """
    border_title = "Regex Flags:"


class RegexFunctions(Widget):
    DEFAULT_CSS = """
    RegexFunctions {
        visibility: hidden;
    }
    """

    def compose(self) -> ComposeResult:
        with RadioSet():
            yield RadioButton("search")
            yield RadioButton("match")


class BaseScreen(Screen):
    BINDINGS = [
        (":", "open_cmd"),
        # ("p", "pattern"),
        # ("i", "input"),
        # ("f", "flags"),
        # ("o", "options"),
    ]

    def action_open_cmd(self):
        self.query_one(CmdInput).visible = True
        self.query_one(CmdInput).focus()

    def compose(self) -> ComposeResult:
        with Vertical(id="Base"):
            with Horizontal(id="Top"):
                yield Header(id="Header")
                yield Logo()
            yield CmdInput()
            # yield PatternInput(id="PatternInput")
            # yield InputArea(id="InputArea")
            # yield RegexOptions(id="RegexOptions")
            # yield RegexFunctions(id="RegexFunctions")


class BaseApp(App):
    def on_ready(self) -> None:
        self.push_screen(BaseScreen())


if __name__ == "__main__":
    app = BaseApp()
    app.run()
