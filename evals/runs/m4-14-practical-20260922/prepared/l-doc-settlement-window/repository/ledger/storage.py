REFUNDS = []


def append_refund(reference, amount):
    record = {"reference": reference, "amount": amount}
    REFUNDS.append(record)
    return record
