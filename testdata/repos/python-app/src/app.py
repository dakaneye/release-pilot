"""A sample Python application."""


def greet(name: str) -> str:
    """Return a greeting for the given name."""
    return f"Hello, {name}!"


def farewell(name: str) -> str:
    """Return a farewell for the given name."""
    return f"Goodbye, {name}!"


if __name__ == "__main__":
    print(greet("world"))
