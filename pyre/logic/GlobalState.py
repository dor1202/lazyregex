import re

from ..metaclasses.singelton import Singleton


class GlobalState(metaclass=Singleton):
    def __init__(self):
        self.pattern = ""
        self.text = ""
        # (group name, position, value)
        self.groups = []
        self.raw_groups = []
        self.regex_options = [
            # name, flag
            ("single_line", re.S),
            ("insensitive", re.I),
        ]
        self.regex_method = "match"
