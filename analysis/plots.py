"""Build report plots from results/<run_id>/*.csv (implemented in stage 11)."""

import argparse
import sys


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", help="path to results/<run_id>")
    parser.parse_args(argv)
    print("plots.py is not implemented yet (stage 11)", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
