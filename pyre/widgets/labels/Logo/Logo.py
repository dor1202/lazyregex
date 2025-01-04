from textual.widgets import Label


class Logo(Label):
    def __init__(self):
        super().__init__(
            """.----..-.  .-..----. .----.\n| {}  }\ \/ / | {}  }| {__/\n| .--'  }  {  | .-. \| {__\n`-'     `--'  `-' `-'`----'""",
            id="Logo",
        )
