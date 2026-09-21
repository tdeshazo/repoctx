from .config import DEFAULT_CURRENCY
from .pricing import late_fee, normalize_reference
from .storage import append_refund


def quote_invoice(reference, amount, days_late, currency=None):
    normalized = normalize_reference(reference)
    return {
        "reference": normalized,
        "currency": currency or DEFAULT_CURRENCY,
        "late_fee": late_fee(days_late, amount),
    }


def record_refund(reference, amount):
    normalized = normalize_reference(reference)
    return append_refund(normalized, amount)
