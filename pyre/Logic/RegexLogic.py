from tokenize import group

from pyre.metaclasses.singelton import Singleton

from functools import reduce

import re


class RegexLogic(metaclass=Singleton):

    # TODO: Add a debounce to the update_pattern and update_text methods

    def __init__(self):
        self.pattern = ""
        self.text = ""
        # (group name, position, value)
        self.groups = []
        self.regex_options = []
        self.regex_method = "match"

    def update_pattern(self, pattern: str):
        self.pattern = pattern
        self._run_regex()
        # self.run_worker(self.update_regex(), exclusive=True)

    def update_text(self, text: str):
        self.text = text
        self._run_regex()
        # self.run_worker(self.update_regex(), exclusive=True)

    def _run_regex(self):
        try:
            if self.regex_options:
                combined_flags = reduce(lambda x, y: x | y, self.regex_options)
                pattern = re.compile(self.pattern, combined_flags)
            else:
                pattern = re.compile(self.pattern)
            method = getattr(pattern, self.regex_method)
            matches = method(self.text)
            if matches:
                self.groups = self._combine_matches_groups(matches)
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
            groups = [("Match 1", f"{matches.pos}-{matches.endpos}", matches.string)]
        return groups
