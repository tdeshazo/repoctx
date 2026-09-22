def late_fee(days_late, amount):
    """Charge one percent per late day, currently capped at ten days."""
    billable_days = min(max(days_late, 0), 10)
    return round(amount * billable_days * 0.01, 2)


def normalize_reference(reference):
    return reference.strip().lower()
