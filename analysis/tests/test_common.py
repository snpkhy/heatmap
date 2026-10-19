"""Tests for reading metrics.csv on a small synthetic file."""

import pandas as pd
import pytest

from common import METRICS_COLUMNS, load_metrics


def test_load_metrics_ok(tmp_path):
    path = tmp_path / "metrics.csv"
    row = dict.fromkeys(METRICS_COLUMNS, 0)
    row.update(run_id="r1", pipeline="p1", f1=0.75)
    pd.DataFrame([row]).to_csv(path, index=False)

    df = load_metrics(path)

    assert len(df) == 1
    assert df.loc[0, "f1"] == pytest.approx(0.75)


def test_load_metrics_missing_columns(tmp_path):
    path = tmp_path / "metrics.csv"
    pd.DataFrame([{"run_id": "r1", "pipeline": "p1"}]).to_csv(path, index=False)

    with pytest.raises(ValueError, match="missing columns"):
        load_metrics(path)
