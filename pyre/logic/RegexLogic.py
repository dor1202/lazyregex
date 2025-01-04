import re
from functools import reduce

from ..logic.GlobalState import GlobalState
from ..metaclasses.singelton import Singleton


class RegexLogic(metaclass=Singleton):

    def __init__(self):
        self.global_state = GlobalState()

    def update_pattern(self, pattern: str):
        GlobalState().pattern = pattern
        self._run_regex()

    def update_text(self, text: str):
        self.global_state.text = text
        self._run_regex()

    def _run_regex(self):
        try:
            if self.global_state.regex_options:
                options = [option[1] for option in self.global_state.regex_options]
                combined_flags = reduce(lambda x, y: x | y, options)
                pattern = re.compile(self.global_state.pattern, combined_flags)
            else:
                pattern = re.compile(self.global_state.pattern)
            # https://stackoverflow.com/questions/11686516/python-regexp-global-flag
            # TODO: add a check of the options, and use the function for it, use finditer for the findall to get indexes
            method = getattr(pattern, self.global_state.regex_method)
            matches = method(self.global_state.text)
            if matches:
                self.global_state.raw_groups = matches.groups() or [matches]
                self.global_state.groups = self._combine_matches_groups(matches)
            else:
                self.global_state.raw_groups = []
                self.global_state.groups = []
        except re.error as e:
            print(e)

    @staticmethod
    def _combine_matches_groups(matches):
        groups = []
        if matches.groupdict():
            for match in matches.groupdict():
                group = []
                for i in range(len(match.groups())):
                    group.append(match.group(i + 1))
                groups.append(group)
        else:
            groups = [
                (
                    "Match 1",
                    f"{matches.regs[0][0]}-{matches.regs[0][1]}",
                    matches.string,
                )
            ]
        return groups
