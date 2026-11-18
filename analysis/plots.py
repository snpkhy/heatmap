"""Build report plots from results/<run_id>/*.csv.

Writes PNGs into <run>/plots/:
  f1_by_sequence.png  F1 per pipeline, one bar per sequence (clean frames)
  f1_by_noise.png     F1 per pipeline, one bar per noise setting
  fps.png             processing speed per pipeline (log scale)
  f1_per_frame.png    F1 over time, one panel per pipeline (from metrics.csv)
"""

import argparse
import sys
from pathlib import Path

import matplotlib

matplotlib.use("Agg")  # no display needed
import matplotlib.pyplot as plt  # noqa: E402
import pandas as pd  # noqa: E402

from common import find_run, load_metrics, load_summary, none_first  # noqa: E402

# Validated categorical slots 1-3 (they stay distinguishable for colour-blind
# readers in every pairing); charts never show more than three series.
SERIES = ["#2a78d6", "#eb6834", "#1baf7a"]
SURFACE = "#fcfcfb"
INK = "#0b0b0b"
INK_2 = "#52514e"
MUTED = "#898781"
GRID = "#e1e0d9"
AXIS = "#c3c2b7"
MAX_SERIES = len(SERIES)


def style(ax: plt.Axes, title: str, ylabel: str) -> None:
    """Recessive axes: hairline grid behind the data, no top/right spines."""
    ax.set_facecolor(SURFACE)
    ax.set_title(title, loc="left", color=INK, fontsize=12)
    ax.set_ylabel(ylabel, color=INK_2)
    ax.tick_params(colors=MUTED, labelcolor=INK_2)
    ax.grid(axis="y", color=GRID, linewidth=0.8)
    ax.set_axisbelow(True)
    for side in ("top", "right"):
        ax.spines[side].set_visible(False)
    for side in ("left", "bottom"):
        ax.spines[side].set_color(AXIS)


def grouped_bars(table: pd.DataFrame, title: str, ylabel: str, path: Path) -> Path:
    """One group per pipeline, one bar per column (at most MAX_SERIES columns)."""
    table = table.iloc[:, :MAX_SERIES]
    fig, ax = plt.subplots(figsize=(9, 4.2), facecolor=SURFACE)
    n = len(table.columns)
    width = 0.8 / max(n, 1)
    for i, col in enumerate(table.columns):
        xs = [x + (i - (n - 1) / 2) * width for x in range(len(table))]
        # A surface-coloured edge leaves a gap between neighbouring bars.
        ax.bar(
            xs, table[col], width, label=str(col), color=SERIES[i], edgecolor=SURFACE, linewidth=2
        )
    ax.set_xticks(range(len(table)), table.index)
    ax.set_ylim(0, 1)
    style(ax, title, ylabel)
    if n >= 2:
        ax.legend(
            frameon=False, labelcolor=INK_2, ncols=n, loc="upper left", bbox_to_anchor=(0, -0.08)
        )
    fig.tight_layout()
    fig.savefig(path, dpi=150, facecolor=SURFACE)
    plt.close(fig)
    return path


def fps_bars(summary: pd.DataFrame, path: Path) -> Path:
    fps = summary.groupby("pipeline")["fps"].mean().sort_index()
    fig, ax = plt.subplots(figsize=(9, 3.6), facecolor=SURFACE)
    ax.bar(fps.index, fps.values, 0.6, color=SERIES[0], edgecolor=SURFACE, linewidth=2)
    ax.set_yscale("log")
    for x, v in enumerate(fps.values):
        ax.annotate(
            f"{v:.0f}",
            (x, v),
            ha="center",
            va="bottom",
            fontsize=8,
            color=INK_2,
            xytext=(0, 2),
            textcoords="offset points",
        )
    style(ax, "Processing speed", "frames per second (log)")
    fig.tight_layout()
    fig.savefig(path, dpi=150, facecolor=SURFACE)
    plt.close(fig)
    return path


def f1_per_frame(metrics: pd.DataFrame, path: Path) -> Path | None:
    """Small multiples: F1 of every evaluated frame, one panel per pipeline."""
    final = metrics[(metrics["stage"] == "final") & metrics["f1"].notna()]
    clean = final[final["noise_type"].isin(["none"]) | final["noise_type"].isna()]
    if len(clean):
        final = clean
    if final.empty:
        return None
    first_seq = sorted(final["sequence"].unique())[0]
    final = final[final["sequence"] == first_seq]
    pipes = sorted(final["pipeline"].unique())
    cols = min(4, len(pipes))
    rows = (len(pipes) + cols - 1) // cols
    fig, axes = plt.subplots(
        rows, cols, figsize=(3 * cols, 2.2 * rows), sharey=True, squeeze=False, facecolor=SURFACE
    )
    for ax, pid in zip(axes.flat, pipes, strict=False):
        d = final[final["pipeline"] == pid].sort_values("frame")
        ax.plot(d["frame"], d["f1"], color=SERIES[0], linewidth=1.5)
        ax.set_ylim(0, 1)
        style(ax, pid, "")
    for ax in list(axes.flat)[len(pipes) :]:
        ax.set_visible(False)
    fig.suptitle(f"F1 per frame — {first_seq}", x=0.01, ha="left", color=INK)
    fig.tight_layout()
    fig.savefig(path, dpi=150, facecolor=SURFACE)
    plt.close(fig)
    return path


def make_plots(run: Path) -> list[Path]:
    """Create all plots for a run directory and return their paths."""
    out = run / "plots"
    out.mkdir(exist_ok=True)
    summary = load_summary(run / "summary.csv")
    made = []
    clean = summary[summary["noise"] == "none"]
    by_seq = (clean if len(clean) else summary).pivot_table(
        index="pipeline", columns="sequence", values="f1", aggfunc="mean"
    )
    made.append(grouped_bars(by_seq, "F1 by sequence (no noise)", "F1", out / "f1_by_sequence.png"))
    by_noise = none_first(
        summary.pivot_table(index="pipeline", columns="noise", values="f1", aggfunc="mean")
    )
    made.append(
        grouped_bars(by_noise, "F1 by noise (mean over sequences)", "F1", out / "f1_by_noise.png")
    )
    made.append(fps_bars(summary, out / "fps.png"))
    metrics_path = run / "metrics.csv"
    if metrics_path.is_file():
        p = f1_per_frame(load_metrics(metrics_path), out / "f1_per_frame.png")
        if p is not None:
            made.append(p)
    return made


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawTextHelpFormatter
    )
    parser.add_argument("--run", help="path to results/<run_id> or an experiment directory")
    parser.add_argument("--results", default="results", help="results root (for the latest run)")
    args = parser.parse_args(argv)
    try:
        run = find_run(args.run, args.results)
    except FileNotFoundError as e:
        print(e, file=sys.stderr)
        return 1
    for p in make_plots(run):
        print(p)
    return 0


if __name__ == "__main__":
    sys.exit(main())
