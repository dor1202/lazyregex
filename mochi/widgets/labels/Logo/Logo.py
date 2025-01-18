from textual.widgets import Label


class Logo(Label):
    def __init__(self):
        super().__init__(
            """_     _.\n _ __ ___   ___   ___| |__ (_)\n| '_ ` _ \ / _ \ / __| '_ \| |\n| | | | | | (_) | (__| | | | |\n|_| |_| |_|\___/ \___|_| |_|_|\n""",
            id="Logo",
        )
