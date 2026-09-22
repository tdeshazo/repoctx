from .config import MAX_BATCH_SIZE


def accepts_batch_size(recipient_count):
    """Return whether a dispatch may include this many recipients."""
    return 0 < recipient_count <= MAX_BATCH_SIZE
