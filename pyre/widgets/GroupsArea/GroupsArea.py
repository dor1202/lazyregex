from textual.app import ComposeResult
from textual.widget import Widget
from textual.widgets import DataTable


class GroupsArea(Widget):
    BORDER_TITLE = "Groups"

    ROWS = [
        ("Group 1", "abc"),
        ("Group 2", "abc"),
    ]

    def compose(self) -> ComposeResult:
        yield DataTable(id="GroupsArea")

    def on_mount(self):
        table = self.query_one(DataTable)
        table.disabled = True
        table.show_cursor = False
        table.cell_padding = 6
        table.add_columns("Group Name", "Match")
        table.add_rows(self.ROWS)
