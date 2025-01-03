from tokenize import group

from pyre.metaclasses.singelton import Singleton

from functools import reduce

import re


class RegexLogic(metaclass=Singleton):

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

    def update_pattern(self, pattern: str):
        self.pattern = pattern
        self._run_regex()

    def update_text(self, text: str):
        self.text = text
        self._run_regex()

    def _run_regex(self):
        try:
            if self.regex_options:
                options = [option[1] for option in self.regex_options]
                combined_flags = reduce(lambda x, y: x | y, options)
                pattern = re.compile(self.pattern, combined_flags)
            else:
                pattern = re.compile(self.pattern)
            # https://stackoverflow.com/questions/11686516/python-regexp-global-flag
            # TODO: add a check of the options, and use the function for it, use finditer for the findall to get indexes
            method = getattr(pattern, self.regex_method)
            matches = method(self.text)
            if matches:
                self.raw_groups = matches.groups() or [matches]
                self.groups = self._combine_matches_groups(matches)
            else:
                self.raw_groups = []
                self.groups = []
        except re.error as e:
            print(e)

    def _combine_matches_groups(self, matches):
        groups = []
        if matches.groupdict():
            for match in matches.groupdict():
                group = []
                for i in range(len(match.groups())):
                    group.append(match.group(i + 1))
                groups.append(group)
        else:
            groups = [("Match 1", f"{matches.regs[0][0]}-{matches.regs[0][1]}", matches.string)]
        return groups
