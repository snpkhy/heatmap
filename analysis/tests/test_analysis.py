"""Tests for compare.py and plots.py on small synthetic result files."""

import pandas as pd
import pytest

import compare
import plots
from common import METRICS_COLUMNS, SUMMARY_COLUMNS, find_run, load_summary


def write_run(path, pipelines=("p1", "p2"), sequences=("synthetic/street", "synthetic/night")):
    """Create summary.csv and metrics.csv like `heatmap experiment` does."""
    path.mkdir(parents=True, exist_ok=True)
    rows, mrows = [], []
    for i, p in enumerate(pipelines):
        for seq in sequences:
            for ntype, level in (("none", 0), ("gaussian", 10)):
                run_id = f"{p}-{seq}-{ntype}"
                f1 = 0.9 - 0.1 * i - (0.2 if ntype == "gaussian" else 0)
                row = dict.fromkeys(SUMMARY_COLUMNS, 0)
                row.update(
                    run_id=run_id,
                    pipeline=p,
                    sequence=seq,
                    noise_type=ntype,
                    noise_level=level,
                    frames=5,
                    precision=f1,
                    recall=f1,
                    f1=f1,
                    iou=f1 / 2,
                    psnr=30,
                    fps=100 * (i + 1),
                )
                rows.append(row)
                for frame in range(5):
                    m = dict.fromkeys(METRICS_COLUMNS, 0)
                    m.update(
                        run_id=run_id,
                        pipeline=p,
                        sequence=seq,
                        noise_type=ntype,
                        noise_level=level,
                        frame=frame,
                        stage="final",
                        f1=f1,
                    )
                    mrows.append(m)
    pd.DataFrame(rows).to_csv(path / "summary.csv", index=False)
    pd.DataFrame(mrows).to_csv(path / "metrics.csv", index=False)
    return path


def test_load_summary_noise_labels(tmp_path):
    run = write_run(tmp_path / "r")
    df = load_summary(run / "summary.csv")
    assert set(df["noise"]) == {"none", "gaussian:10"}


def test_find_run_latest_and_errors(tmp_path):
    with pytest.raises(FileNotFoundError):
        find_run(None, tmp_path)
    with pytest.raises(FileNotFoundError):
        find_run(tmp_path / "missing")
    old = write_run(tmp_path / "old")
    new = write_run(tmp_path / "new")
    (tmp_path / "no-summary").mkdir()
    import os

    os.utime(old, (1, 1))
    assert find_run(None, tmp_path) == new
    assert find_run(old) == old


def test_build_tables(tmp_path):
    df = load_summary(write_run(tmp_path / "r") / "summary.csv")
    tables = compare.build_tables(df)
    by_seq = tables["F1 by sequence (no noise)"]
    assert list(by_seq.index) == ["p1", "p2"]
    assert by_seq.loc["p1", "synthetic/street"] == pytest.approx(0.9)
    by_noise = tables["F1 by noise (mean over sequences)"]
    assert by_noise.loc["p2", "gaussian:10"] == pytest.approx(0.6)
    assert tables["Speed, frames per second (mean)"].loc["p2", "fps"] == pytest.approx(200)


def test_to_markdown_handles_nan():
    df = pd.DataFrame({"a": [0.5, float("nan")]}, index=pd.Index(["p1", "p2"], name="pipeline"))
    md = compare.to_markdown(df)
    assert md.splitlines()[0] == "| pipeline | a |"
    assert "| p1 | 0.500 |" in md
    assert "| p2 | — |" in md


def test_compare_main_writes_files(tmp_path, capsys):
    run = write_run(tmp_path / "exp")
    assert compare.main(["--run", str(run)]) == 0
    assert "F1 by noise" in (run / "comparison.md").read_text(encoding="utf-8")
    assert (run / "comparison.csv").is_file()
    assert "Pipeline comparison" in capsys.readouterr().out
    assert compare.main(["--run", str(tmp_path / "missing")]) == 1


def test_plots_main_creates_pngs(tmp_path):
    run = write_run(tmp_path / "exp")
    assert plots.main(["--run", str(run)]) == 0
    for name in ("f1_by_sequence.png", "f1_by_noise.png", "fps.png", "f1_per_frame.png"):
        f = run / "plots" / name
        assert f.is_file() and f.stat().st_size > 1000, name
    assert plots.main(["--results", str(tmp_path / "nothing")]) == 1


def test_plots_without_metrics_and_many_series(tmp_path):
    run = write_run(tmp_path / "r", pipelines=("p1",), sequences=("a", "b", "c", "d"))
    (run / "metrics.csv").unlink()
    made = plots.make_plots(run)
    assert [p.name for p in made] == ["f1_by_sequence.png", "f1_by_noise.png", "fps.png"]
