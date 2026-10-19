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


def load_metrics(path: Path | str) -> pd.DataFrame:
    """Read metrics.csv and check that all required columns are present.

    Raises ValueError listing the missing columns if the file is incomplete.
    """
    df = pd.read_csv(path)
    missing = [c for c in METRICS_COLUMNS if c not in df.columns]
    if missing:
        raise ValueError(f"{path} is missing columns: {', '.join(missing)}")
    return df
