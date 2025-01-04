from textual.app import ComposeResult
from textual.reactive import reactive
from textual.widget import Widget
from textual.widgets import DataTable

from rich.text import Text
from rich.style import Style


class GroupsArea(Widget):
    BORDER_TITLE = "Groups"

    # [Group Name, Position, Match]
    groups = reactive([])

    def compose(self) -> ComposeResult:
        yield DataTable(id="GroupsArea")

    def on_mount(self):
        table = self.query_one(DataTable)
        table.disabled = True
        table.show_cursor = False
        table.cell_padding = 4
        table.add_columns("Name", "Position", "Match")

    def watch_groups(self, value):
        table = self.query_one(DataTable)
        table.clear()

        for row in value:
            # Adding styled and justified `Text` objects instead of plain strings.
            color = "green" if "Match" in row[0] else "yellow"
            colored_col = Text(
                str(row[0]),
                style=Style(underline=True, color=color),
            )
            styled_row = [Text(str(cell)) for cell in row[1:]]
            table.add_row(colored_col, *styled_row)
