def format_label(tracking_code, city):
    destination = city.strip().title()[:20]
    return f"{tracking_code} | {destination}"
