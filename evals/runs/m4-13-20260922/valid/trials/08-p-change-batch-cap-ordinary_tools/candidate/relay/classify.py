from .config import DEFAULT_QUEUE


def choose_queue(region, expedited):
    """Choose the dispatch queue for a report."""
    if expedited and region == "north":
        return "north-priority"
    if expedited:
        return "priority"
    return DEFAULT_QUEUE
