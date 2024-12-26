from textual.app import ComposeResult
from textual.widget import Widget

from ...widgets.GeneralData.GeneralData import GeneralData
from ...widgets.Logo.Logo import Logo


class CustomHeader(Widget):
    DEFAULT_CSS = """
    CustomHeader {
        height: 4;
        layout: horizontal;
    }
    """

    def __init__(self):
        super().__init__(id="CustomHeader")

    def compose(self) -> ComposeResult:
        yield GeneralData()
        yield Logo()
