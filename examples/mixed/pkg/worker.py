from pathlib import Path

class Worker:
    def __init__(self, root: str):
        self.root = Path(root)

    def files(self):
        return list(self.root.glob("**/*"))

def count_files(root: str) -> int:
    return len(Worker(root).files())
