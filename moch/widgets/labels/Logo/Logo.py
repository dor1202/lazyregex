from textual.widgets import Label


class Logo(Label):
    def __init__(self):
        super().__init__(
            """____________________________\n_ __ ___   ___   ___| |____\n| '_ ` _ \ / _ \ / __| '_ \|\n| | | | | | (_) | (__| | | |\n|_| |_| |_|\___/ \___|_| |_|\n""",
            id="Logo",
        )
