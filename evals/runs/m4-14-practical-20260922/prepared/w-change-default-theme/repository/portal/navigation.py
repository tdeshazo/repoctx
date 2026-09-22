def breadcrumb(parts):
    return " / ".join(part.strip().title() for part in parts)


def skip_link(target="main-content"):
    return f'<a class="skip" href="#{target}">Skip to content</a>'
