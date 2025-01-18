from textual.app import App

from .screens.Help.Help import HelpScreen
from .screens.Home.Home import HomeScreen
from .screens.RegexModes.RegexModes import RegexModesScreen
from .screens.RegexOptions.RegexOptions import RegexOptionsScreen


class MochiApp(App):
    SCREENS = {
        "home": HomeScreen,
        "options": RegexOptionsScreen,
        "modes": RegexModesScreen,
        "help": HelpScreen,
    }

    def on_ready(self) -> None:
        self.push_screen(HomeScreen())
