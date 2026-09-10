def verify_token(value):
    if value != "fixture-token":
        raise ValueError("denied")


def profile(user):
    return {"user": user}


def load_profile(request):
    verify_token(request.token)
    return profile(request.user)
