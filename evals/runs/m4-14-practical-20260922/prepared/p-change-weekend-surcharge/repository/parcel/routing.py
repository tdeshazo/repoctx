from .config import DEFAULT_LANE, WEEKEND_SURCHARGE


def choose_lane(region, hazardous=False, requested=None):
    if hazardous:
        return "ground"
    if requested:
        return requested
    return "north-n1" if region == "north" else DEFAULT_LANE


def delivery_price(base, weekend=False):
    return base + (WEEKEND_SURCHARGE if weekend else 0)
