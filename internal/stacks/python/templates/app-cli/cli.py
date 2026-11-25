"""Command-line entry point. Usage: python cli.py [name]. Prints a greeting; --help prints the usage."""

import sys

USAGE = "usage: cli.py [name]"


def main(args: list) -> int:
    if "--help" in args or "-h" in args:
        print(USAGE)
        return 0
    if len(args) > 1:
        print(USAGE, file=sys.stderr)
        return 2
    print(f"Hello, {args[0] if args else 'world'}!")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
