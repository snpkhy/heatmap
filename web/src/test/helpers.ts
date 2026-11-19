import { vi } from "vitest";
import type { PipelineInfo, Run, RunListItem, SequenceInfo, Summary } from "../api";

type Handler = (init?: RequestInit) => unknown;

/** Replaces fetch with canned JSON responses keyed by "METHOD path". */
export function mockFetch(routes: Record<string, unknown | Handler>) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push({ url, init });
    const key = `${init?.method ?? "GET"} ${url}`;
    if (!(key in routes)) {
      return new Response(JSON.stringify({ error: `no mock for ${key}` }), { status: 404 });
    }
    const v = routes[key];
    const body = typeof v === "function" ? (v as Handler)(init) : v;
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fn);
  return calls;
}

export const pipelines: PipelineInfo[] = [
  {
    id: "p1",
    name: "Baseline",
    owner: "Illia",
    description: "Gaussian blur and a background model.",
    stages: [{ method: "gaussian" }, { method: "running_avg_otsu" }, { method: "closing" }],
    stage_info: [
      { key: "1-gaussian", method: "gaussian", category: "filtering", denoise: true },
      {
        key: "2-running_avg_otsu",
        method: "running_avg_otsu",
        category: "segmentation",
        denoise: false,
      },
      { key: "3-closing", method: "closing", category: "morphology", denoise: false },
    ],
  },
  { id: "p9", name: "Broken", stages: [], stage_info: null, error: "unknown method" },
];

/** A second working detection mode, for tests that switch between modes. */
export const shadows: PipelineInfo = { ...pipelines[0], id: "p7", name: "Shadows" };

export const sequences: SequenceInfo[] = [
  {
    id: "synthetic/street",
    name: "street",
    category: "synthetic",
    source: "synthetic",
    path: "Road with a car",
    frames: 150,
    width: 160,
    height: 120,
    has_gt: true,
    eval_start: 10,
    eval_end: 149,
  },
];

export function summary(over: Partial<Summary> = {}): Summary {
  return {
    run_id: "r1",
    pipeline: "p1",
    pipeline_name: "Baseline",
    sequence: "synthetic/street",
    category: "synthetic",
    noise: { type: "none", level: 0, seed: 1 },
    start: 0,
    frames: 3,
    eval_frames: 2,
    width: 160,
    height: 120,
    precision: 0.9,
    recall: 0.8,
    f1: 0.847,
    iou: 0.73,
    psnr: 30.1,
    ssim: 0.91,
    fps: 512.3,
    total_ms: 5.9,
    stages: [
      { key: "1-gaussian", method: "gaussian", category: "filtering", mean_ms: 0.5 },
      {
        key: "2-running_avg_otsu",
        method: "running_avg_otsu",
        category: "segmentation",
        mean_ms: 1.2,
      },
    ],
    has_plan: false,
    created_at: "2026-11-19T10:00:00Z",
    ...over,
  };
}

export function run(over: Partial<Summary> = {}): Run {
  return {
    summary: summary(over),
    pipeline: pipelines[0],
    stage_keys: ["1-gaussian", "2-running_avg_otsu"],
    saved_frames: true,
    frames: [0, 1, 2].map((i) => ({
      frame: i,
      has_gt: i > 0,
      precision: i > 0 ? 0.9 : null,
      recall: i > 0 ? 0.8 : null,
      f1: i > 0 ? 0.85 : null,
      iou: i > 0 ? 0.7 : null,
      psnr: 30,
      time_ms: 2,
      moving: 0.05,
    })),
  };
}

export function listItem(id: string, over: Partial<RunListItem> = {}): RunListItem {
  return {
    run_id: id,
    status: "done",
    summary: summary({ run_id: id }),
    created_at: "2026-11-19T10:00:00Z",
    ...over,
  };
}
