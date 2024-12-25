import platform

from textual import events
from textual.app import App, ComposeResult
from textual.screen import Screen
from textual.widgets import Footer, Static, Input, Label, TextArea, RadioButton, SelectionList, RadioSet, Header
from textual.containers import Horizontal, Vertical, Grid
from textual.widget import Widget
from textual.widgets.selection_list import Selection


class Logo(Static):
    DEFAULT_CSS = """
    Logo {
        width: 30%;
        height: 5;
        color: orange;
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


class SideData(Widget):
    DEFAULT_CSS = """
    SideData {
        width: 80%;
        height: 5;
    }
    
    Grid {
        grid-size: 2 4;
    }
    
    Label {
        content-align: center middle;
    }
    
    .GridDesc {
        color: orange;
    }
    """

    def __init__(self):
        super().__init__(id="Header")

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
        super().__init__(id="CmdInput", disabled=True)

    def action_close_input(self):
        self.visible = False
        self.disabled = True
        self.value = ""

    def focus_pattern(self):
        self.action_close_input()
        self.app.query_one(PatternInput).disabled = False
        self.app.query_one(PatternInput).focus()

    def focus_input(self):
        self.action_close_input()
        self.app.query_one(InputArea).disabled = False
        self.app.query_one(InputArea).focus()

    def open_modes(self):
        self.action_close_input()
        self.app.push_screen("modes")

    def open_options(self):
        self.action_close_input()
        self.app.push_screen("options")

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
        }
        com = commands.get(self.value, self.action_close_input)
        com()


class PatternInput(Input):
    BORDER_TITLE = "Pattern"

    DEFAULT_CSS = """
    PatternInput {
        width: 100%;
        border-title-color: white;
        border: blue;
    }
    """

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="PatternInput", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True


class InputArea(TextArea):

    BORDER_TITLE = "Test String"

    DEFAULT_CSS = """
    InputArea {
        width: 100%;
        border-title-color: white;
        border: blue;
    }
    """

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def __init__(self):
        super().__init__(id="InputArea", disabled=True)

    def action_drop_focus_input(self):
        self.disabled = True


class RegexOptions(Widget):

    BORDER_TITLE = "Regex Options"

    def compose(self) -> ComposeResult:
        yield SelectionList(
            Selection("Global (g)", "global"),
            Selection("Multi Line (m)", "multi_line"),
            Selection("Insensitive (i)", "insensitive"),
            Selection("Extended (x)", "extended"),
            Selection("Single Line (s)", "single_line"),
            Selection("Unicode (u)", "unicode"),
            Selection("Ascii (a)", "ascii"),
        )

class OptionsScreen(Screen):
    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield Label("Regex Options")
        yield RegexOptions()


class RegexModes(Widget):

    def compose(self) -> ComposeResult:
        with RadioSet():
            yield RadioButton("match")
            yield RadioButton("substitution")
            yield RadioButton("search")

class ModesScreen(Screen):
    BINDINGS = [
        ("escape", "back_to_main"),
    ]

    def action_back_to_main(self):
        self.app.pop_screen()

    def compose(self) -> ComposeResult:
        yield Label("Regex Modes")
        yield RegexModes()

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


class CustomFooter(Label):
    pass

class BaseScreen(Screen):
    BINDINGS = [
        (":", "open_cmd"),
    ]

    def action_open_cmd(self):
        self.query_one(CmdInput).visible = True
        self.query_one(CmdInput).disabled = False
        self.query_one(CmdInput).focus()

    def compose(self) -> ComposeResult:
        # with Vertical(id="Base"):
        with Horizontal(id="Top"):
            yield SideData()
            yield Logo()
        yield CmdInput()
        yield PatternInput()
        yield InputArea()
        yield CustomFooter()


class BaseApp(App):
    SCREENS = {
        "main": BaseScreen,
        "options": OptionsScreen,
        "modes": ModesScreen
    }

    def on_ready(self) -> None:
        self.push_screen(BaseScreen())


if __name__ == "__main__":
    app = BaseApp()
    app.run()
