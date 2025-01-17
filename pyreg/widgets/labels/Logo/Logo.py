from textual.widgets import Label


class Logo(Label):
    def __init__(self):
        super().__init__(
            """._ __  _   _ _ __ ___  __ _.\n| '_ \| | | | '__/ _ \/ _` |\n| |_) | |_| | | |  __/ (_| |\n| .__/ \__, |_|  \___|\__, |\n|_|    |___/          |___/.""",
            id="Logo",
        )
