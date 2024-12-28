from textual.containers import Horizontal
from textual.widgets import Label
from textual.reactive import reactive


class RegexPattern(Horizontal):
    DEFAULT_CSS = """
    RegexPattern {
        border: white;
    }
    
    .Blue {
        color: blue;
    }
    
    .Pink {
        color: pink;
    }
    
    .Brown {
        color: brown;
    }
    
    .Yellow {
        color: yellow;
    }
    
    .Purple {
        color: purple;
    }
    
    .Orange {
        color: orange;
    }
    
    .Green {
        color: green;
    }
    """

    REGEX_COLORS = {
        "^": "Blue",
        "[": "Pink",
        "]": "Pink",
        "-": "Brown",
        ".": "Yellow",
        "*": "Purple",
        "+": "Purple",
        "?": "Purple",
        "(": "Orange",
        ")": "Orange",
        "|": "Orange",
        "{": "Orange",
        "}": "Orange",
        ",": "Orange",
    }

    value = reactive("^[a-z|1|2]?{1,4}", recompose=True)

    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    # def compose(self):
    #     """Compose the child widgets for the input."""
    #     for char in self.value:
    #         yield Label(char, classes=f"{self.get_color(char)}")

    def get_color(self, char: str) -> str:
        if char.isdigit():
            return "Blue"
        if char.isalpha():
            return "Green"
        return self.REGEX_COLORS.get(char, "White")

    def watch_value(self, new_value: str):
        """Update the input value and re-render the children."""
        self.value = new_value
        self.remove_children()  # Clear existing labels
        self.mount_all(Label(char, classes=f"{self.get_color(char)}") for char in self.value)

    async def focus(self, scroll_visible: bool = True):
        """Handle when the widget gains focus."""
        self.border_title = "Focused"
        self.refresh()

    async def blur(self):
        """Handle when the widget loses focus."""
        self.border_title = ""
        self.refresh()

    # async def on_event(self, key):
    #     """Handle key presses to update the input."""
    #     if event.character:
    #         # Append the character to the value
    #         self.update_value(self.value + event.character)
    #     elif event.key == "backspace":
    #         # Remove the last character
    #         self.update_value(self.value[:-1])
    #     raise Exception("Unhandled event")




