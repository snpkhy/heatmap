import { quantile } from "./engine";

// FrameStore downloads what the viewer plays: for every frame of a run the
// footage and the motion mask the server saved. Masks are reduced to one
// byte per pixel on a grid no wider than MAX_GRID, which is all the heat
// layer needs and keeps a 500-frame run within a few dozen megabytes.

const MAX_GRID = 384;
const PARALLEL = 8;

/** The heat grid for a scene: its own size, or scaled down to MAX_GRID wide. */
export function gridSize(width: number, height: number): [number, number] {
  if (width <= MAX_GRID) return [Math.max(1, width), Math.max(1, height)];
  return [MAX_GRID, Math.max(1, Math.round((height * MAX_GRID) / width))];
}

export class FrameStore {
  readonly gridW: number;
  readonly gridH: number;
  /** undefined while loading, null when the file could not be loaded. */
  readonly images: (HTMLImageElement | null | undefined)[];
  readonly masks: (Uint8Array | undefined)[];
  /** Share of the frame that moves, per frame. */
  readonly activity: Float32Array;
  /** Share of the frame that has moved at least once, up to each frame. */
  readonly coverage: Float32Array;
  /** How often each pixel moved over the frames loaded so far. */
  readonly totals: Uint16Array;
  /** The count shown as hottest in the cumulative heatmap. */
  peak = 1;
  /** Frames [0, ready) are loaded and can be played. */
  ready = 0;

  private settled: boolean[];
  private available = 0;
  private next = 0;
  private inflight = 0;
  private disposed = false;
  private covered = 0;
  private ctx: CanvasRenderingContext2D | null = null;

  constructor(
    /** URL of a file of the frame at position i (0-based within the run). */
    private readonly url: (i: number, file: "input.png" | "mask.png") => string,
    readonly length: number,
    width: number,
    height: number,
    private readonly onChange: () => void,
  ) {
    [this.gridW, this.gridH] = gridSize(width, height);
    this.images = new Array(length);
    this.masks = new Array(length);
    this.settled = new Array(length).fill(false);
    this.activity = new Float32Array(length);
    this.coverage = new Float32Array(length);
    this.totals = new Uint16Array(this.gridW * this.gridH);
    if (typeof document !== "undefined") {
      const canvas = document.createElement("canvas");
      canvas.width = this.gridW;
      canvas.height = this.gridH;
      this.ctx = canvas.getContext("2d", { willReadFrequently: true });
    }
  }

  /** Frames [0, n) exist on the server and may be fetched. */
  setAvailable(n: number) {
    this.available = Math.max(this.available, Math.min(n, this.length));
    this.pump();
  }

  get complete(): boolean {
    return this.ready === this.length;
  }

  /** Stops fetching; resume() continues where it stopped. */
  dispose() {
    this.disposed = true;
  }

  resume() {
    this.disposed = false;
    this.pump();
  }

  private pump() {
    while (!this.disposed && this.inflight < PARALLEL && this.next < this.available) {
      this.load(this.next++);
    }
  }

  private load(i: number) {
    this.inflight++;
    const image = fetchImage(this.url(i, "input.png"));
    const mask = fetchImage(this.url(i, "mask.png"));
    Promise.all([image, mask]).then(([img, m]) => {
      this.inflight--;
      this.images[i] = img;
      this.masks[i] = m ? this.reduce(m) : undefined;
      this.settled[i] = true;
      this.advance();
      this.pump();
    });
  }

  /** Scales a mask image to the grid and keeps one byte (0 or 1) per pixel. */
  private reduce(img: HTMLImageElement): Uint8Array | undefined {
    const { ctx, gridW, gridH } = this;
    if (!ctx) return undefined;
    ctx.clearRect(0, 0, gridW, gridH);
    ctx.drawImage(img, 0, 0, gridW, gridH);
    const data = ctx.getImageData(0, 0, gridW, gridH).data;
    const out = new Uint8Array(gridW * gridH);
    for (let i = 0, p = 0; i < out.length; i++, p += 4) out[i] = data[p] >= 128 ? 1 : 0;
    return out;
  }

  /** Extends the playable prefix and the statistics that depend on frame order. */
  private advance() {
    const before = this.ready;
    const { totals, masks } = this;
    while (this.ready < this.length && this.settled[this.ready]) {
      const mask = masks[this.ready];
      let moving = 0;
      if (mask) {
        for (let p = 0; p < mask.length; p++) {
          if (mask[p]) {
            moving++;
            if (totals[p]++ === 0) this.covered++;
          }
        }
      }
      this.activity[this.ready] = moving / totals.length;
      this.coverage[this.ready] = this.covered / totals.length;
      this.ready++;
    }
    if (this.ready !== before) {
      this.peak = Math.max(1, quantile(totals, 0.99));
      this.onChange();
    }
  }
}

function fetchImage(src: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    const img = new Image();
    img.decoding = "async";
    img.onload = () => resolve(img);
    img.onerror = () => resolve(null);
    img.src = src;
  });
}
