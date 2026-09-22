from .config import DEFAULT_THEME
from .navigation import breadcrumb, skip_link


def render_page(title, sections, theme=None):
    trail = breadcrumb(["docs", title])
    links = "".join(f"<li>{section}</li>" for section in sections)
    return (
        skip_link()
        + f'<main id="main-content" data-theme="{theme or DEFAULT_THEME}">'
        + f"<nav>{trail}</nav><h1>{title}</h1><ul>{links}</ul></main>"
    )
