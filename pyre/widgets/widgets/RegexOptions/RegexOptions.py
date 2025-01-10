import re

from textual import on
from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import SelectionList
from textual.widgets.selection_list import Selection

from ....logic.GlobalState import GlobalState


class RegexOptions(Widget):
    BORDER_TITLE = "Regex Options"

    OPTIONS = [
        # (name, value, flag)
        ("Global", "global", "global"),
        ("Multi Line (m)", "multi_line", re.M),
        ("Insensitive (i)", "insensitive", re.I),
        ("Extended (x)", "extended", re.X),
        ("Single Line (s)", "single_line", re.S),
        ("Unicode (u)", "unicode", re.U),
        ("Ascii (a)", "ascii", re.A),
    ]

    def compose(self) -> ComposeResult:
        option_names = [option[1] for option in GlobalState().regex_options]
        selections = [
            Selection(element[0], element[1], element[2] in option_names)
            for element in self.OPTIONS
        ]
        yield SelectionList(*selections)

    @on(SelectionList.SelectedChanged)
    def update_selected_view(self) -> None:
        selected = self.query_one(SelectionList).selected

        select_data_full_row = [
            option
            for select in selected
            for option in self.OPTIONS
            if select == option[1]
        ]
        GlobalState().regex_options = [(option[1], option[2]) for option in select_data_full_row]

        from ..FooterOptions.FooterOptions import FooterOptions

        main_screen = self.app.screen_stack[-2]
        main_screen.query_one(FooterOptions).options = self.query_one(
            SelectionList
        ).selected
