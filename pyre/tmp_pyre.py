from textual.app import App

from .screens.Home.Home import HomeScreen
from .screens.RegexModes.RegexModes import RegexModesScreen
from .screens.RegexOptions.RegexOptions import RegexOptionsScreen


class PyreApp(App):
    SCREENS = {
        "home": HomeScreen,
        "options": RegexOptionsScreen,
        "modes": RegexModesScreen
    }

    def on_ready(self) -> None:
        self.push_screen(HomeScreen())
