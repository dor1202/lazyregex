from textual.app import ComposeResult
from textual.widget import Widget
from textual.containers import Container

from ...widgets.GeneralData.GeneralData import GeneralData
from ...widgets.Logo.Logo import Logo
from ...widgets.Help.Help import Help


class CustomHeader(Widget):
    DEFAULT_CSS = """
    CustomHeader {
        height: 5;
        layout: horizontal;
    }
    """

    def __init__(self):
        super().__init__(id="CustomHeader")

    def compose(self) -> ComposeResult:
        yield Container(GeneralData())
        yield Container(Help())
        yield Container(Logo())
