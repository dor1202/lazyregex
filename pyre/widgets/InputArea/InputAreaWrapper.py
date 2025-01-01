import re
from textual.widgets import Static, TextArea
from textual.reactive import reactive
from textual.app import ComposeResult
from rich.text import Text

from .InputArea import InputArea


class InputAreaWrapper(Static):
    """
    A Textual widget that provides a text input area with dynamic regex-based highlighting.

    Attributes:
        regex_pattern (str): The regex pattern to match and highlight.
    """

    content = reactive("")

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self.regex_pattern = re.compile("a")  # Compile the regex pattern
        self.text_area = InputArea()  # Main input area
        self.highlighted_display = Static()  # Highlighted output display

    def compose(self) -> ComposeResult:
        """
        Compose the widget by adding the text area and the output display.
        """
        yield self.text_area
        yield self.highlighted_display

    def watch_content(self, new_value: str) -> None:
        """
        Watches for changes in the text area's content and updates the highlighted display.
        """
        highlighted_text = self.apply_regex_highlight(new_value)
        self.highlighted_display.update(highlighted_text)

    async def on_key(self, event) -> None:
        """
        Captures key events to update the content as the user types.
        """
        if self.text_area.has_focus:
            self._content = self.text_area.text  # Update content dynamically

    def apply_regex_highlight(self, text: str) -> Text:
        """
        Apply regex-based highlighting to the provided text.

        Args:
            text (str): The input text to be processed.

        Returns:
            Text: A Rich Text object with highlighted matches.
        """
        rich_text = Text()
        last_pos = 0

        for match in self.regex_pattern.finditer(text):
            start, end = match.span()
            # Append non-matched text
            rich_text.append(text[last_pos:start])
            # Append matched text with styling
            rich_text.append(text[start:end], style="bold red")
            last_pos = end

        # Append the remaining text
        rich_text.append(text[last_pos:])
        return rich_text
