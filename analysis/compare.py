"""Compare pipelines across sequences and noise levels.

Reads summary.csv of a run or experiment directory and writes comparison.md
and comparison.csv next to it: mean F1 per pipeline and sequence (clean
frames), mean F1 per pipeline and noise setting, and speed.
"""

import argparse
import sys
from pathlib import Path

import pandas as pd

from common import find_run, load_summary, none_first


def pivot(df: pd.DataFrame, columns: str, value: str = "f1") -> pd.DataFrame:
    """Mean of ``value`` per pipeline (rows) and ``columns``."""
    return df.pivot_table(
        index="pipeline", columns=columns, values=value, aggfunc="mean"
    ).sort_index()


def to_markdown(df: pd.DataFrame, digits: int = 3) -> str:
    """Render a small table as GitHub markdown without extra dependencies."""
    header = [df.index.name or ""] + [str(c) for c in df.columns]
    lines = ["| " + " | ".join(header) + " |", "|" + "---|" * len(header)]
    for idx, row in df.iterrows():
        cells = ["—" if pd.isna(v) else f"{v:.{digits}f}" for v in row]
        lines.append("| " + " | ".join([str(idx), *cells]) + " |")
    return "\n".join(lines)


def build_tables(summary: pd.DataFrame) -> dict[str, pd.DataFrame]:
    """The comparison tables, keyed by title."""
    clean = summary[summary["noise"] == "none"]
    tables = {
        "F1 by sequence (no noise)": pivot(clean if len(clean) else summary, "sequence"),
        "F1 by noise (mean over sequences)": none_first(pivot(summary, "noise")),
        "Precision / recall / IoU (mean over all runs)": summary.groupby("pipeline")[
            ["precision", "recall", "iou"]
        ]
        .mean()
        .sort_index(),
        "Speed, frames per second (mean)": summary.groupby("pipeline")[["fps"]].mean().sort_index(),
    }
    return tables


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", help="path to results/<run_id> or an experiment directory")
    parser.add_argument("--results", default="results", help="results root (for the latest run)")
    args = parser.parse_args(argv)
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")  # Windows consoles default to cp1252
    try:
        run = find_run(args.run, args.results)
    except FileNotFoundError as e:
        print(e, file=sys.stderr)
        return 1

    summary = load_summary(run / "summary.csv")
    tables = build_tables(summary)
    parts = [f"# Pipeline comparison — {run.name}", f"{len(summary)} runs."]
    for title, table in tables.items():
        digits = 1 if "Speed" in title else 3
        parts += [f"## {title}", to_markdown(table, digits)]
    text = "\n\n".join(parts) + "\n"
    Path(run, "comparison.md").write_text(text, encoding="utf-8")
    pd.concat(tables, names=["table"]).to_csv(run / "comparison.csv")
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
