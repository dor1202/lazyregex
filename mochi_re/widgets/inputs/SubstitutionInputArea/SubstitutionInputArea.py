from textual.widgets import TextArea
from textual.reactive import reactive

from ....logic.GlobalState import GlobalState


class SubstitutionInputArea(TextArea):
    DEFAULT_CSS = """
    SubstitutionInputArea {
        width: 100%;
    }
    
    SubstitutionInputArea:disabled {
        opacity: 100% !important;
    }
    """

    BORDER_TITLE = "Substitution Output"

    output_text = reactive("")

    def __init__(self, *args, **kwargs):
        super().__init__(disabled=True, *args, **kwargs)

    def watch_output_text(self, value):
        self.text = value
        self._add_colors()

    def highlight(
            self, row: int, start_column: int, end_column: int, color: str
    ) -> None:
        self._highlights[row].append((start_column, end_column, color))

    def _add_colors(self):
        if not GlobalState().groups:
            return

        sub_length = len(GlobalState().substitution_input)
        for index, (name, position, _) in enumerate(GlobalState().groups):
            start, _ = position.split("-")
            start = int(start)
            calc_start = (index * (sub_length - 1)) + start
            self.highlight(0, calc_start, calc_start + sub_length, "cyan")
