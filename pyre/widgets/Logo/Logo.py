from textual.widgets import Label


class Logo(Label):
    def __init__(self):
        super().__init__(id="Logo")
        logo_path = "pyre/widgets/Logo/Logo.txt"
        with open(logo_path, "r") as f:
            self.renderable = f.read()
