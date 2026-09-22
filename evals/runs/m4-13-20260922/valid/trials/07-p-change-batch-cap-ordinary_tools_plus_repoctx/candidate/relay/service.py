from .batches import accepts_batch_size
from .classify import choose_queue


def prepare_dispatch(report_id, region, expedited, recipient_count):
    """Validate a report and return its dispatch details."""
    if not accepts_batch_size(recipient_count):
        raise ValueError("recipient batch is outside the accepted range")
    return {
        "report_id": report_id,
        "queue": choose_queue(region, expedited),
    }
