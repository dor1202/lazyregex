import re

from rich.highlighter import RegexHighlighter, _combine_regex
from rich.style import Style
from rich.text import Span, Text


class PatternHighlighter(RegexHighlighter):
    """Highlights regular expressions"""

    # Define regex patterns for different components of regular expressions
    REGEX_GROUP = r"(?P<group>\((?:\?:|\?=|\?!|\?<=|\?<!|\?>|[^?])*?\))"
    REGEX_CHAR_CLASS = r"(?P<char_class>\[.*?\])"
    REGEX_ESCAPE = r"(?P<escape>\\[wsd])"
    REGEX_QUANTIFIER = r"(?P<quantifier>\{\d+(,\d*)?\}|\*|\+|\?)"
    REGEX_META = r"(?P<meta>[.^$|])"

    highlights = [
        _combine_regex(
            REGEX_GROUP,
            REGEX_CHAR_CLASS,
            REGEX_ESCAPE,
            REGEX_QUANTIFIER,
            REGEX_META,
        )
    ]

    def highlight(self, text: Text) -> None:
        super().highlight(text)

        # Additional work to handle nested or complex regex structures
        plain = text.plain
        append = text.spans.append
        for match in re.finditer(self.REGEX_GROUP, plain):
            start, end = match.span()
            append(Span(start, end, Style(color="green")))
        for match in re.finditer(self.REGEX_CHAR_CLASS, plain):
            start, end = match.span()
            append(Span(start, end, Style(color="blue")))
        for match in re.finditer(self.REGEX_ESCAPE, plain):
            start, end = match.span()
            append(Span(start, end, Style(color="orange1")))
        for match in re.finditer(self.REGEX_QUANTIFIER, plain):
            start, end = match.span()
            append(Span(start, end, Style(color="cyan")))
        for match in re.finditer(self.REGEX_META, plain):
            start, end = match.span()
            append(Span(start, end, Style(color="blue")))
