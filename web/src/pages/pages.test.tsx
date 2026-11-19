import { ThemeProvider } from "@mui/material/styles";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { listItem, mockFetch, pipelines, run, sequences, shadows, summary } from "../test/helpers";
import { theme } from "../theme";
import { Compare, runLabel } from "./Compare";
import { History } from "./History";
import { madeWith, Scenes } from "./Scenes";
import { Viewer } from "./Viewer";

function renderWithTheme(ui: ReactNode) {
  return render(
    <ThemeProvider theme={theme} defaultMode="dark">
      {ui}
    </ThemeProvider>,
  );
}

/** The interface speaks of scenes and detection modes, never of pipelines. */
function expectPlainWording() {
  expect(document.body.textContent).not.toMatch(/pipeline/i);
}

describe("Scenes", () => {
  it("shows the scenes and analyses the one that is picked", async () => {
    const calls = mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/sequences": sequences,
      "GET /api/runs": [],
      "POST /api/runs": { run_id: "new-run" },
    });
    renderWithTheme(<Scenes />);

    expect(await screen.findByRole("heading", { name: "Street" })).toBeDefined();
    expect(screen.getByText("Road with a car")).toBeDefined();
    expect(screen.getByText("Built-in")).toBeDefined();
    expect(screen.getByAltText("Preview of synthetic/street").getAttribute("src")).toContain(
      "frame=79",
    );
    // The first working detection mode is preselected, by its name.
    expect(screen.getByText("Baseline")).toBeDefined();
    expectPlainWording();

    fireEvent.click(screen.getByRole("button", { name: "Analyze Street" }));
    await waitFor(() => expect(window.location.hash).toBe("#/runs/new-run"));
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(String(post?.init?.body))).toEqual({
      pipeline: "p1",
      sequence: "synthetic/street",
      noise: "none",
      seed: 1,
      frames: 300,
    });
  });

  it("offers to watch a scene that was already analysed", async () => {
    mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/sequences": sequences,
      "GET /api/runs": [listItem("run-new"), listItem("run-old")],
    });
    renderWithTheme(<Scenes />);
    const watch = await screen.findByRole("link", { name: "Watch Street" });
    expect(watch.getAttribute("href")).toBe("#/runs/run-new");
    expect(screen.getByRole("button", { name: "Analyze Street again" })).toBeDefined();
  });

  it("remembers the chosen mode and only offers results made with it", async () => {
    // The scene was analysed with the first mode; the second one is selected.
    localStorage.setItem("heatmap.settings", JSON.stringify({ mode: "p7", frames: 150 }));
    const calls = mockFetch({
      "GET /api/pipelines": [...pipelines, shadows],
      "GET /api/sequences": sequences,
      "GET /api/runs": [listItem("run-a")],
      "POST /api/runs": { run_id: "new-run" },
    });
    renderWithTheme(<Scenes />);
    expect(await screen.findByText("Shadows")).toBeDefined();
    expect(screen.queryByRole("link", { name: "Watch Street" })).toBeNull();

    // Back to the first mode: its result is offered again, and the choice is saved.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Detection mode" }));
    fireEvent.click(screen.getByRole("option", { name: "Baseline" }));
    expect(screen.getByRole("link", { name: "Watch Street" })).toBeDefined();
    expect(JSON.parse(localStorage.getItem("heatmap.settings") ?? "{}").mode).toBe("p1");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Detection mode" }));
    fireEvent.click(screen.getByRole("option", { name: "Shadows" }));
    fireEvent.click(screen.getByRole("button", { name: "Analyze Street" }));
    await waitFor(() => expect(window.location.hash).toBe("#/runs/new-run"));
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(String(post?.init?.body))).toMatchObject({ pipeline: "p7", frames: 150 });
  });

  it("tells analyses apart by mode and noise", () => {
    const s = summary();
    expect(madeWith(s, "p1", "none", 10)).toBe(true);
    expect(madeWith(s, "p7", "none", 10)).toBe(false);
    expect(madeWith(s, "p1", "gaussian", 10)).toBe(false);
    const noisy = summary({ noise: { type: "gaussian", level: 10, seed: 1 } });
    expect(madeWith(noisy, "p1", "gaussian", 10)).toBe(true);
    expect(madeWith(noisy, "p1", "gaussian", 20)).toBe(false);
    expect(madeWith(noisy, "p1", "none", 10)).toBe(false);
  });

  it("reports API errors", async () => {
    mockFetch({ "GET /api/sequences": sequences });
    renderWithTheme(<Scenes />);
    expect(await screen.findByText(/no mock for GET \/api\/pipelines/)).toBeDefined();
  });
});

describe("History", () => {
  it("lists analyses and enables comparing two finished ones", async () => {
    mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/runs": [
        listItem("run-a"),
        listItem("run-b"),
        listItem("run-c", {
          status: "running",
          summary: undefined,
          progress: { run_id: "run-c", done: 4, total: 9, frame: 3, fps: 50 },
          request: {
            pipeline: "p1",
            sequence: "synthetic/street",
            noise: "none",
            seed: 1,
            frames: 9,
          },
        }),
        listItem("run-d", { status: "failed", summary: undefined, error: "boom" }),
      ],
    });
    renderWithTheme(<History />);
    const table = await screen.findByRole("table", { name: "analyses" });
    expect(within(table).getAllByRole("row")).toHaveLength(5);
    expect(within(table).getAllByText("0.847")).toHaveLength(2);
    expect(within(table).getByText("failed")).toBeDefined();
    expect(within(table).getByText("4/9")).toBeDefined();
    // A run in progress is named by its detection mode too.
    await waitFor(() => expect(within(table).getAllByText("Baseline")).toHaveLength(3));
    expectPlainWording();

    const compare = screen.getByRole("button", { name: /compare/i });
    expect(compare.hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByLabelText("select run-a"));
    fireEvent.click(screen.getByLabelText("select run-b"));
    expect(compare.hasAttribute("disabled")).toBe(false);
    fireEvent.click(compare);
    expect(window.location.hash).toBe("#/compare?ids=run-a,run-b");
  });

  it("shows a hint when nothing was analysed", async () => {
    mockFetch({ "GET /api/runs": [], "GET /api/pipelines": pipelines });
    renderWithTheme(<History />);
    expect(await screen.findByText(/Nothing analysed yet/)).toBeDefined();
  });

  it("deletes an analysis", async () => {
    let deleted = false;
    mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/runs": () => (deleted ? [] : [listItem("run-a")]),
      "DELETE /api/runs/run-a": () => {
        deleted = true;
        return { status: "deleted" };
      },
    });
    renderWithTheme(<History />);
    fireEvent.click(await screen.findByLabelText("delete run-a"));
    expect(await screen.findByText(/Nothing analysed yet/)).toBeDefined();
  });
});

describe("Viewer", () => {
  it("plays a finished analysis: stage, controls, numbers and details", async () => {
    mockFetch({ "GET /api/runs/r1": { run_id: "r1", status: "done", run: run() } });
    renderWithTheme(<Viewer id="r1" />);

    expect(await screen.findByRole("heading", { name: "Street" })).toBeDefined();
    expect(screen.getByRole("img", { name: "Footage with the heat layer" })).toBeDefined();
    expect(screen.getByText("0.847")).toBeDefined(); // accuracy
    expect(screen.getByText("512 fps")).toBeDefined();
    expect(screen.getByText("30.1 dB")).toBeDefined();
    expect(screen.getByText("Moving now")).toBeDefined();
    expectPlainWording();

    // It plays on its own; the button pauses and resumes.
    fireEvent.click(screen.getByRole("button", { name: "pause" }));
    expect(screen.getByRole("button", { name: "play" })).toBeDefined();
    expect(screen.getByRole("slider", { name: "frame" })).toBeDefined();

    // Layers: trails by default, with a palette and a trail length.
    expect(screen.getByRole("button", { name: "Trails" }).getAttribute("aria-pressed")).toBe(
      "true",
    );
    expect(screen.getByRole("slider", { name: "trail length" }).hasAttribute("disabled")).toBe(
      false,
    );
    fireEvent.click(screen.getByRole("button", { name: "Heatmap" }));
    expect(screen.getByRole("slider", { name: "trail length" }).hasAttribute("disabled")).toBe(
      true,
    );
    fireEvent.click(screen.getByRole("button", { name: "Thermal palette" }));
    expect(
      screen.getByRole("button", { name: "Thermal palette" }).getAttribute("aria-pressed"),
    ).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: "Video only" }));
    expect(screen.queryByText("busy")).toBeNull(); // no heat, no legend

    // The steps of the frame on screen.
    expect(screen.getByAltText("motion mask").getAttribute("src")).toBe(
      "/api/runs/r1/files/frames/0/mask.png",
    );
    expect(screen.getByAltText("1. gaussian")).toBeDefined();
    expect(screen.queryByAltText("ground truth")).toBeNull(); // frame 0 has none
    expect(screen.queryByAltText(/floor plan/)).toBeNull();

    fireEvent.click(screen.getByRole("tab", { name: "Charts" }));
    expect(screen.getByText("Accuracy per frame")).toBeDefined();
    expect(screen.getByText("Time per step (ms per frame)")).toBeDefined();
  });

  it("shows the same frames through another detection mode", async () => {
    const calls = mockFetch({
      "GET /api/runs/r1": { run_id: "r1", status: "done", run: run() },
      "GET /api/pipelines": [...pipelines, shadows],
      "GET /api/runs": [listItem("r1")],
      "POST /api/runs": { run_id: "fresh" },
    });
    renderWithTheme(<Viewer id="r1" />);
    // No analysis with that mode yet: one is started on the same frames.
    fireEvent.mouseDown(await screen.findByRole("combobox", { name: "Detection mode" }));
    fireEvent.click(screen.getByRole("option", { name: "Shadows" }));
    await waitFor(() => expect(window.location.hash).toBe("#/runs/fresh"));
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(String(post?.init?.body))).toEqual({
      pipeline: "p7",
      sequence: "synthetic/street",
      noise: "none",
      seed: 1,
      start: 0,
      frames: 3,
    });
  });

  it("opens the analysis of another mode when it already exists", async () => {
    const other = listItem("r7", { summary: summary({ run_id: "r7", pipeline: "p7" }) });
    const calls = mockFetch({
      "GET /api/runs/r1": { run_id: "r1", status: "done", run: run() },
      "GET /api/pipelines": [...pipelines, shadows],
      "GET /api/runs": [listItem("r1"), other],
    });
    renderWithTheme(<Viewer id="r1" />);
    fireEvent.mouseDown(await screen.findByRole("combobox", { name: "Detection mode" }));
    fireEvent.click(screen.getByRole("option", { name: "Shadows" }));
    await waitFor(() => expect(window.location.hash).toBe("#/runs/r7"));
    expect(calls.some((c) => c.init?.method === "POST")).toBe(false);
  });

  it("shows the floor plan and the directions when the analysis has them", async () => {
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
    renderWithTheme(<Viewer id="r1" />);
    expect(await screen.findByText("→ right")).toBeDefined();
    expect(screen.getByAltText("Heatmap on the floor plan").getAttribute("src")).toBe(
      "/api/runs/r1/files/plan_heatmap.png",
    );
    fireEvent.click(screen.getByRole("tab", { name: "Charts" }));
    expect(screen.getByText("Where movement goes (6 vectors)")).toBeDefined();
  });

  it("follows an analysis that is still running", async () => {
    mockFetch({
      "GET /api/runs/live": {
        run_id: "live",
        status: "running",
        progress: { run_id: "live", done: 5, total: 10, frame: 4, fps: 99 },
        live: {
          request: {
            pipeline: "p1",
            sequence: "synthetic/street",
            noise: "none",
            seed: 1,
            frames: 10,
          },
          scene: sequences[0],
          start: 0,
        },
      },
    });
    renderWithTheme(<Viewer id="live" />);
    expect(await screen.findByText(/Frame 5 of 10/)).toBeDefined();
    expect(screen.getByRole("heading", { name: "Street" })).toBeDefined();
    expect(screen.getByText("running")).toBeDefined();
    expect(screen.getByRole("progressbar", { name: "run progress" })).toBeDefined();
    // Nothing to scrub or play until it is done.
    expect(screen.getByRole("button", { name: "play" }).hasAttribute("disabled")).toBe(true);
  });

  it("shows why an analysis failed, and one that waits in the queue", async () => {
    mockFetch({
      "GET /api/runs/bad": { run_id: "bad", status: "failed", error: "frame 3: boom" },
      "GET /api/runs/q": {
        run_id: "q",
        status: "queued",
        progress: { run_id: "q", done: 0, total: 0 },
      },
    });
    renderWithTheme(<Viewer id="bad" />);
    expect(await screen.findByText("frame 3: boom")).toBeDefined();
    renderWithTheme(<Viewer id="q" />);
    expect(await screen.findByText("Waiting in queue…")).toBeDefined();
  });

  it("shows the final heatmap of an analysis saved without frames", async () => {
    const r = { ...run(), saved_frames: false };
    mockFetch({ "GET /api/runs/r1": { run_id: "r1", status: "done", run: r } });
    renderWithTheme(<Viewer id="r1" />);
    expect(await screen.findByText(/saved without its frames/)).toBeDefined();
    expect(screen.getByAltText("heatmap over the scene").getAttribute("src")).toBe(
      "/api/runs/r1/files/overlay.png",
    );
    expect(screen.getByText("0.847")).toBeDefined();
    expect(screen.queryByRole("slider", { name: "frame" })).toBeNull();
  });

  it("reports a missing analysis", async () => {
    mockFetch({});
    renderWithTheme(<Viewer id="nope" />);
    expect(await screen.findByText(/no mock/)).toBeDefined();
  });
});

describe("Compare", () => {
  it("compares the selected analyses", async () => {
    mockFetch({
      "GET /api/runs": [
        listItem("a"),
        listItem("b", {
          summary: summary({ run_id: "b", pipeline: "p7", pipeline_name: "Shadows", f1: 0.96 }),
        }),
        listItem("c"),
      ],
    });
    renderWithTheme(<Compare ids={["a", "b"]} />);
    const table = await screen.findByRole("table", { name: "comparison" });
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(within(table).getByText("0.960")).toBeDefined();
    expect(within(table).getByText("Shadows")).toBeDefined();
    expect(screen.getByText("Accuracy")).toBeDefined();
    expect(screen.getByAltText("Heatmap of Street · Shadows").getAttribute("src")).toBe(
      "/api/runs/b/files/heatmap.png",
    );
    expectPlainWording();
  });

  it("explains when there is nothing to compare", async () => {
    mockFetch({ "GET /api/runs": [] });
    renderWithTheme(<Compare ids={[]} />);
    expect(await screen.findByText(/Nothing to compare yet/)).toBeDefined();
  });

  it("labels analyses compactly", () => {
    const s = summary();
    expect(runLabel(s)).toBe("Street · Baseline");
    expect(runLabel({ ...s, noise: { type: "gaussian", level: 10, seed: 1 } })).toBe(
      "Street · Baseline · gaussian 10",
    );
  });
});
