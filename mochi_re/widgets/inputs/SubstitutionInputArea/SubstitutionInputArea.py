from textual.widgets import TextArea
from textual.reactive import reactive


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

