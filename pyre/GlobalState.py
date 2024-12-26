from .metaclasses.singelton import Singleton


class GlobalState(metaclass=Singleton):
    def __init__(self):
        self.regex_options = set(["aaa", "bbbb"])
        self.regex_method = "match"
