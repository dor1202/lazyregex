import re
from functools import reduce
from typing import Iterator

from ..logic.GlobalState import GlobalState
from ..metaclasses.singelton import Singleton


class RegexLogic(metaclass=Singleton):

    def __init__(self):
        self.global_state = GlobalState()

    def update_pattern(self, pattern: str):
        GlobalState().pattern = pattern
        self._run_regex()
        if GlobalState().regex_method == "substitution":
            self._run_substitution()

    def update_text(self, text: str):
        self.global_state.text = text
        self._run_regex()
        if GlobalState().regex_method == "substitution":
            self._run_substitution()
    
    def update_substitution_input(self, substitution_input: str):
        self.global_state.substitution_input = substitution_input
        self._run_regex()
        self._run_substitution()

    def _run_substitution(self):
        regex = self._calc_regex_element()
        results = regex.sub(self.global_state.substitution_input, self.global_state.text)
        self.global_state.substitution_output = results

    def _run_regex(self):
        regex = self._calc_regex_element()
        results = self._run_regex_method(regex)
        self._handle_regex_results(results)
    
    def _calc_regex_element(self):
        try:
            if self.global_state.regex_options:
                options = [option[1] for option in self.global_state.regex_options if option[0] != "global"]
                combined_flags = reduce(lambda x, y: x | y, options)
                pattern = re.compile(self.global_state.pattern, combined_flags)
            else:
                pattern = re.compile(self.global_state.pattern)
            
            return pattern

        except re.error as e:
            self.global_state.groups = []
            print(e)
    
    def _run_regex_method(self, pattern: re.Pattern[str]):
        if any(option[0] == "global" for option in self.global_state.regex_options):
            return pattern.finditer(self.global_state.text)
        else:
            method = getattr(pattern, self.global_state.regex_method)
            return method(self.global_state.text)

    def _handle_regex_results(self, results):
        if isinstance(results, re.Match):
            self.global_state.groups = self._combine_matches_groups([results])
        elif isinstance(results, Iterator):
            self.global_state.groups = self._combine_matches_groups(results)
        else:
            self.global_state.groups = []

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
