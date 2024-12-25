from textual.widget import Widget
from textual.app import ComposeResult
from textual.widgets import SelectionList
from textual.widgets.selection_list import Selection


class RegexOptions(Widget):
    CSS_PATH = "RegexOptions.tcss"

    BORDER_TITLE = "Regex Options"

    def compose(self) -> ComposeResult:
        options = [
            ("Global (g)", "global"),
            ("Multi Line (m)", "multi_line"),
            ("Insensitive (i)", "insensitive"),
            ("Extended (x)", "extended"),
            ("Single Line (s)", "single_line"),
            ("Unicode (u)", "unicode"),
            ("Ascii (a)", "ascii"),
        ]

        selections = [Selection(*element) for element in options]
        yield SelectionList(*selections)
