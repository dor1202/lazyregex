from textual.app import ComposeResult
from textual.containers import Grid
from textual.containers import Container
from textual.widget import Widget

from ....widgets.labels.Logo.Logo import Logo
from ...widgets.GeneralData.GeneralData import GeneralData
from ...widgets.Help.Help import Help


class CustomHeader(Widget):
    DEFAULT_CSS = """
    CustomHeader {
        height: 5;
        layout: horizontal;
    }
    
    Grid{
        grid-size: 3 1;
        grid-gutter: 1 2;
    }
    
    Grid GeneralData {
      column-span: 1;
    }
    
    Grid Help {
      column-span: 1;
    }
    
    Grid Logo {
      column-span: 1;
    }
    """

    def __init__(self):
        super().__init__(id="CustomHeader")

    def compose(self) -> ComposeResult:
        yield Grid(GeneralData(), Help(), Logo())
