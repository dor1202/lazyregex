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
            matches_iterator = pattern.finditer(self.global_state.text)
            # method = getattr(pattern, self.global_state.regex_method)
            # matches = method(self.global_state.text)
            if matches_iterator:
                self.global_state.groups = self._combine_matches_groups(matches_iterator)
            else:
                self.global_state.groups = []
        except re.error as e:
            self.global_state.groups = []
            print(e)

    @staticmethod
    def _combine_matches_groups(matches_iterator):
        groups = []
        for index, match in enumerate(matches_iterator):
            groups.append(
                (
                    f"Match {index}",
                    f"{match.start()}-{match.end()}",
                    match.group(0),
                )
            )
            for group_name, group_match in match.groupdict().items():
                if group_match is not None:  # Check if the group has a match
                    start, end = match.span(group_name)
                    groups.append(
                        (
                            f"Group {group_name}",
                            f"{start}-{end}",
                            group_match,
                        )
                    )
        return groups
