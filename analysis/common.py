"""Shared helpers for reading run outputs (results/<run_id>/*.csv)."""

from pathlib import Path

import pandas as pd

# Minimum set of metrics.csv columns.
METRICS_COLUMNS = [
    "run_id",
    "pipeline",
    "owner",
    "sequence",
    "category",
    "noise_type",
    "noise_level",
    "frame",
    "stage",
    "mse",
    "psnr",
    "ssim",
    "precision",
    "recall",
    "f1",
    "iou",
    "time_ms",
]

# Minimum set of summary.csv columns (one row per run).
SUMMARY_COLUMNS = [
    "run_id",
    "pipeline",
    "sequence",
    "noise_type",
    "noise_level",
    "frames",
    "precision",
    "recall",
    "f1",
    "iou",
    "psnr",
    "fps",
]


def _check(df: pd.DataFrame, columns: list[str], path: Path | str) -> pd.DataFrame:
    missing = [c for c in columns if c not in df.columns]
    if missing:
        raise ValueError(f"{path} is missing columns: {', '.join(missing)}")
    return df


def load_metrics(path: Path | str) -> pd.DataFrame:
    """Read metrics.csv and check that all required columns are present.

    Raises ValueError listing the missing columns if the file is incomplete.
    """
    return _check(pd.read_csv(path), METRICS_COLUMNS, path)


def load_summary(path: Path | str) -> pd.DataFrame:
    """Read summary.csv (one row per run) and add a readable ``noise`` label."""
    df = _check(pd.read_csv(path), SUMMARY_COLUMNS, path)
    df["noise"] = noise_labels(df)
    return df


def noise_labels(df: pd.DataFrame) -> pd.Series:
    """Label noise settings as "none", "gaussian:10", "salt_pepper:0.02"."""

    def label(row: pd.Series) -> str:
        if row["noise_type"] in ("", "none") or pd.isna(row["noise_type"]):
            return "none"
        return f"{row['noise_type']}:{row['noise_level']:g}"

    return df.apply(label, axis=1)


def none_first(table: pd.DataFrame) -> pd.DataFrame:
    """Put the clean "none" column first, so noisy settings read against it."""
    cols = sorted(table.columns, key=lambda c: (c != "none", str(c)))
    return table[cols]


def find_run(path: Path | str | None, results: Path | str = "results") -> Path:
    """Resolve the run or experiment directory to analyse.

    With no path, the most recently modified directory under ``results``
    that has a summary.csv is used.
    """
    if path is not None:
        p = Path(path)
        if not (p / "summary.csv").is_file():
            raise FileNotFoundError(f"{p} has no summary.csv")
        return p
    candidates = [d for d in Path(results).glob("*") if (d / "summary.csv").is_file()]
    if not candidates:
        raise FileNotFoundError(f"no runs with summary.csv under {results}/")
    return max(candidates, key=lambda d: d.stat().st_mtime)
