// Typed client for the Go API (internal/server). All detection happens on the
// server; the UI starts runs and shows what these endpoints return.

export interface ParamSpec {
  name: string;
  type: "int" | "float" | "bool" | "string" | "points";
  default: unknown;
  help: string;
}

export interface MethodInfo {
  name: string;
  category: string;
  description: string;
  denoise: boolean;
  params: ParamSpec[];
}

export interface StageInfo {
  key: string;
  method: string;
  category: string;
  denoise: boolean;
}

export interface PipelineInfo {
  id: string;
  name: string;
  owner?: string;
  description?: string;
  stages: { method: string; params?: Record<string, unknown> }[];
  stage_info: StageInfo[] | null;
  error?: string;
}

export interface SequenceInfo {
  id: string;
  name: string;
  category: string;
  source: "cdnet" | "frames" | "synthetic";
  path?: string;
  frames: number;
  width: number;
  height: number;
  has_gt: boolean;
  eval_start: number;
  eval_end: number;
}

export interface NoiseSpec {
  type: string;
  level: number;
  seed: number;
}

export interface StageTiming {
  key: string;
  method: string;
  category: string;
  mean_ms: number;
}

/** Metrics are null when they could not be computed (no ground truth). */
export interface Summary {
  run_id: string;
  pipeline: string;
  pipeline_name: string;
  owner?: string;
  sequence: string;
  category: string;
  noise: NoiseSpec;
  start: number;
  frames: number;
  eval_frames: number;
  width: number;
  height: number;
  precision: number | null;
  recall: number | null;
  f1: number | null;
  iou: number | null;
  psnr: number | null;
  ssim: number | null;
  fps: number;
  total_ms: number;
  stages: StageTiming[];
  extra?: Record<string, DirectionHistogram | Record<string, unknown>>;
  has_plan: boolean;
  created_at: string;
}

export interface DirectionHistogram {
  type: "direction_histogram";
  bins: number[];
  normalized: number[];
  bin_centers: number[];
  vectors: number;
  dominant_deg?: number;
}

export interface FrameResult {
  frame: number;
  has_gt: boolean;
  precision: number | null;
  recall: number | null;
  f1: number | null;
  iou: number | null;
  psnr: number | null;
  time_ms: number;
  moving: number;
}

export interface Run {
  summary: Summary;
  pipeline: PipelineInfo;
  stage_keys: string[];
  frames: FrameResult[];
  saved_frames: boolean;
}

export type RunStatus = "queued" | "running" | "done" | "failed" | "cancelled";

export interface Progress {
  run_id: string;
  done: number;
  total: number;
  frame: number;
  fps: number;
}

export interface RunRequest {
  pipeline: string;
  sequence: string;
  noise: string;
  seed: number;
  start?: number;
  frames: number;
}

export interface RunListItem {
  run_id: string;
  status: RunStatus;
  error?: string;
  progress?: Progress;
  summary?: Summary;
  request?: RunRequest;
  created_at: string;
}

/** A run started by this server, before its results exist. */
export interface LiveInfo {
  request: RunRequest;
  scene: SequenceInfo;
  /** The first frame of the run; frames are saved under frames/<n>/. */
  start: number;
}

export interface RunDetail {
  run_id: string;
  status: RunStatus;
  error?: string;
  progress?: Progress;
  live?: LiveInfo;
  run?: Run;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError((body as { error?: string }).error ?? res.statusText, res.status);
  }
  return body as T;
}

export const api = {
  methods: () => request<MethodInfo[]>("/api/methods"),
  pipelines: () => request<PipelineInfo[]>("/api/pipelines"),
  sequences: () => request<SequenceInfo[]>("/api/sequences"),
  runs: () => request<RunListItem[]>("/api/runs"),
  run: (id: string) => request<RunDetail>(`/api/runs/${encodeURIComponent(id)}`),
  startRun: (req: RunRequest) =>
    request<{ run_id: string }>("/api/runs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    }),
  deleteRun: (id: string) =>
    request<{ status: string }>(`/api/runs/${encodeURIComponent(id)}`, { method: "DELETE" }),
};

/** URL of a file inside a run directory (heatmap.png, frames/12/mask.png, …). */
export function runFile(id: string, file: string): string {
  return `/api/runs/${encodeURIComponent(id)}/files/${file}`;
}

/** URL of a preview frame of a sequence. */
export function previewURL(sequence: string, frame: number): string {
  return `/api/sequences/preview?id=${encodeURIComponent(sequence)}&frame=${frame}`;
}

export function eventsURL(id: string): string {
  return `/api/runs/${encodeURIComponent(id)}/events`;
}
