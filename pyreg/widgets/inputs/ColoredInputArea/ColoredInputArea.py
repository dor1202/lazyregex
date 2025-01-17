import sys

from rich._palettes import EIGHT_BIT_PALETTE, STANDARD_PALETTE, WINDOWS_PALETTE
from rich.color import ANSI_COLOR_NAMES, Color
from rich.style import Style
from textual.widgets import TextArea

from ....logic.Debouncer import Debouncer
from ....logic.GlobalState import GlobalState
from ....logic.RegexLogic import RegexLogic
from ....widgets.widgets.GroupsArea.GroupsArea import GroupsArea
from ....widgets.inputs.SubstitutionInputArea.SubstitutionInputArea import SubstitutionInputArea

IS_WINDOWS = sys.platform == "win32"

MATCH_COLOR = "cyan"
PREV_MATCH_COLOR = "orange1"
GROUP_COLORs = [
    "green",
    "yellow",
    "purple",
    "pink",
    "red",
    "turquoise",
]


class ColoredInputArea(TextArea):
    DEFAULT_CSS = """
    ColoredInputArea {
        width: 50%;
    }
    
    ColoredInputArea:disabled {
        opacity: 100% !important;
    }
    """

    BORDER_TITLE = "Test String"

    BINDINGS = [
        ("escape", "drop_focus_input"),
    ]

    def action_drop_focus_input(self):
        self.disabled = True

        from ....widgets.widgets.Help.Help import Help
        GlobalState().help_ui = [
            ("<Shift + :>", "Commands Input"),
        ]
        self.app.query_one(Help).help_labels = GlobalState().help_ui

    def __init__(self, *args, **kwargs):
        super().__init__(disabled=True, *args, **kwargs)
        self.debouncer = Debouncer(0.5)
        rich_colors = sorted((v, k) for k, v in ANSI_COLOR_NAMES.items())

        for color_number, name in rich_colors:
            palette = self.get_current_pallet(color_number)
            color = palette[color_number]
            self._theme.syntax_styles[name] = Style(color=Color.from_rgb(*color))

    def highlight(
        self, row: int, start_column: int, end_column: int, color: str
    ) -> None:
        self._highlights[row].append((start_column, end_column, color))

    @staticmethod
    def get_current_pallet(color_number):
        if IS_WINDOWS and color_number < 16:
            return WINDOWS_PALETTE
        return STANDARD_PALETTE if color_number < 16 else EIGHT_BIT_PALETTE

    async def on_text_area_changed(self):
        await self.debouncer.debounce(self.process_input)

    def _calc_color(self, name: str, group_index: int, row: int, start_column: int):
        print(name, group_index, row, start_column)
        print(self._highlights[row])
        prev_color = next((color for start, end, color in self._highlights[row] if end == start_column), None)
        print(prev_color)
        if "match" in name.lower():
            color = MATCH_COLOR
        else:
            color = GROUP_COLORs[group_index % len(GROUP_COLORs)]
        print(color)
        if prev_color and prev_color == color:
            color = PREV_MATCH_COLOR
        print(color)
        return color

    def process_input(self):
        RegexLogic().update_text(self.text)
        self.app.query_one(GroupsArea).groups = GlobalState().groups
        self._highlights.clear()

        if not GlobalState().groups:
            return

        group_index = 0
        for index, (name, position, _) in enumerate(GlobalState().groups):
            start, end = position.split("-")
            start = int(start)
            end = int(end)
            if "match" in name.lower():
                group_index -= 1
            group_index += 1
            self.highlight(0, start, end, self._calc_color(name, group_index, 0, start))
        
        if GlobalState().regex_method == "substitution":
            self.app.query_one(SubstitutionInputArea).output_text = GlobalState().substitution_output
