from .labels import format_label
from .routing import choose_lane, delivery_price


def prepare_parcel(tracking_code, city, region, base_price, hazardous=False):
    return {
        "label": format_label(tracking_code, city),
        "lane": choose_lane(region, hazardous=hazardous),
        "price": delivery_price(base_price),
    }
