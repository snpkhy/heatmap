import { ThemeProvider } from "@mui/material/styles";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { listItem, mockFetch, pipelines, run, sequences, summary } from "../test/helpers";
import { theme } from "../theme";
import { Compare, runLabel } from "./Compare";
import { Launcher } from "./Launcher";
import { RunPage } from "./RunPage";
import { RunsPage } from "./RunsPage";

function renderWithTheme(ui: ReactNode) {
  return render(<ThemeProvider theme={theme}>{ui}</ThemeProvider>);
}

describe("Launcher", () => {
  it("shows pipelines and sequences and starts a run", async () => {
    const calls = mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/sequences": sequences,
      "POST /api/runs": { run_id: "new-run" },
    });
    renderWithTheme(<Launcher />);

    // The first valid pipeline and sequence are preselected.
    expect(await screen.findByText("gaussian · filtering")).toBeDefined();
    expect(screen.getByText("Gaussian blur and a background model.")).toBeDefined();
    expect(screen.getByAltText("Preview of synthetic/street").getAttribute("src")).toContain(
      "frame=79",
    );

    fireEvent.click(screen.getByRole("button", { name: /start run/i }));
    await waitFor(() => expect(window.location.hash).toBe("#/runs/new-run"));
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(String(post?.init?.body))).toEqual({
      pipeline: "p1",
      sequence: "synthetic/street",
      noise: "none",
      seed: 1,
      frames: 150,
    });
  });

  it("reports API errors", async () => {
    mockFetch({ "GET /api/sequences": sequences });
    renderWithTheme(<Launcher />);
    expect(await screen.findByText(/no mock for GET \/api\/pipelines/)).toBeDefined();
  });
});

describe("RunsPage", () => {
  it("lists runs and enables comparing two finished ones", async () => {
    mockFetch({
      "GET /api/runs": [
        listItem("run-a"),
        listItem("run-b"),
        listItem("run-c", {
          status: "failed",
          summary: undefined,
          error: "boom",
          request: undefined,
        }),
      ],
    });
    renderWithTheme(<RunsPage />);
    const table = await screen.findByRole("table", { name: "runs" });
    expect(within(table).getAllByRole("row")).toHaveLength(4);
    expect(within(table).getAllByText("0.847")).toHaveLength(2);
    expect(within(table).getByText("failed")).toBeDefined();

    const compare = screen.getByRole("button", { name: /compare/i });
    expect(compare.hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByLabelText("select run-a"));
    fireEvent.click(screen.getByLabelText("select run-b"));
    expect(compare.hasAttribute("disabled")).toBe(false);
    fireEvent.click(compare);
    expect(window.location.hash).toBe("#/compare?ids=run-a,run-b");
  });

  it("shows a hint when there are no runs", async () => {
    mockFetch({ "GET /api/runs": [] });
    renderWithTheme(<RunsPage />);
    expect(await screen.findByText(/No runs yet/)).toBeDefined();
  });

  it("deletes a run", async () => {
    let deleted = false;
    mockFetch({
      "GET /api/runs": () => (deleted ? [] : [listItem("run-a")]),
      "DELETE /api/runs/run-a": () => {
        deleted = true;
        return { status: "deleted" };
      },
    });
    renderWithTheme(<RunsPage />);
    fireEvent.click(await screen.findByLabelText("delete run-a"));
    expect(await screen.findByText(/No runs yet/)).toBeDefined();
  });
});

describe("RunPage", () => {
  it("shows a finished run: tiles, frames, heatmap and metrics", async () => {
    mockFetch({ "GET /api/runs/r1": { run_id: "r1", status: "done", run: run() } });
    renderWithTheme(<RunPage id="r1" />);

    expect(await screen.findByText("0.847")).toBeDefined(); // F1 tile
    expect(screen.getByText("512 fps")).toBeDefined();
    expect(screen.getByText("30.1 dB")).toBeDefined();

    // Frames tab starts at the first frame with ground truth.
    expect(screen.getByAltText("ground truth").getAttribute("src")).toBe(
      "/api/runs/r1/files/frames/1/gt.png",
    );
    expect(screen.getByAltText("1. gaussian")).toBeDefined();
    expect(screen.getByAltText("motion mask")).toBeDefined();

    fireEvent.click(screen.getByRole("tab", { name: "Heatmap" }));
    expect(screen.getByAltText("activity heatmap over the scene").getAttribute("src")).toBe(
      "/api/runs/r1/files/overlay.png",
    );
    expect(screen.queryByAltText(/floor plan/)).toBeNull();

    fireEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(screen.getByText("Mask quality per frame")).toBeDefined();
    expect(screen.getByText("Mean time per stage (ms)")).toBeDefined();
  });

  it("shows the floor plan and the direction histogram when present", async () => {
    const r = run({
      has_plan: true,
      extra: {
        "3-direction_histogram": {
          type: "direction_histogram",
          bins: [5, 1, 0, 0, 0, 0, 0, 0],
          normalized: [],
          bin_centers: [0, 45, 90, 135, 180, 225, 270, 315],
          vectors: 6,
          dominant_deg: 0,
        },
      },
    });
    mockFetch({ "GET /api/runs/r1": { run_id: "r1", status: "done", run: r } });
    renderWithTheme(<RunPage id="r1" />);
    expect(await screen.findByText("→ right")).toBeDefined();
    fireEvent.click(screen.getByRole("tab", { name: "Heatmap" }));
    expect(screen.getByAltText(/floor plan/)).toBeDefined();
    fireEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(screen.getByText("Motion directions (6 vectors)")).toBeDefined();
  });

  it("shows progress of a running run and errors", async () => {
    mockFetch({
      "GET /api/runs/live": {
        run_id: "live",
        status: "running",
        progress: { run_id: "live", done: 5, total: 10, frame: 4, fps: 99 },
      },
      "GET /api/runs/bad": { run_id: "bad", status: "failed", error: "frame 3: boom" },
    });
    renderWithTheme(<RunPage id="live" />);
    expect(await screen.findByText(/Frame 5 of 10/)).toBeDefined();
    expect(screen.getByAltText("live mask.png").getAttribute("src")).toBe(
      "/api/runs/live/files/frames/4/mask.png",
    );

    renderWithTheme(<RunPage id="bad" />);
    expect(await screen.findByText("frame 3: boom")).toBeDefined();
  });

  it("reports a missing run", async () => {
    mockFetch({});
    renderWithTheme(<RunPage id="nope" />);
    expect(await screen.findByText(/no mock/)).toBeDefined();
  });
});

describe("Compare", () => {
  it("compares the selected runs", async () => {
    mockFetch({
      "GET /api/runs": [
        listItem("a"),
        listItem("b", { summary: summary({ run_id: "b", pipeline: "p7", f1: 0.96 }) }),
        listItem("c"),
      ],
    });
    renderWithTheme(<Compare ids={["a", "b"]} />);
    const table = await screen.findByRole("table", { name: "comparison" });
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(within(table).getByText("0.960")).toBeDefined();
    expect(screen.getByText("Mask quality")).toBeDefined();
  });

  it("explains when there is nothing to compare", async () => {
    mockFetch({ "GET /api/runs": [] });
    renderWithTheme(<Compare ids={[]} />);
    expect(await screen.findByText(/Nothing to compare yet/)).toBeDefined();
  });

  it("labels runs compactly", () => {
    const s = summary();
    expect(runLabel(s)).toBe("p1 · street");
    expect(runLabel({ ...s, noise: { type: "gaussian", level: 10, seed: 1 } })).toBe(
      "p1 · street · gaussian 10",
    );
  });
});
