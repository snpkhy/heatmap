// The heat layer of the viewer: it folds the motion masks of a run into a
// heat field, frame by frame, and turns that field into pixels.
//
//   trail  ← decay · trail + mask      fades, so it follows whoever is moving
//   counts ← counts + mask             never fades: where people have been so far
//
// Nothing here detects motion — the masks come from the server. This only
// decides how they are shown.

export type Palette = "inferno" | "turbo" | "ice";

type Stop = [t: number, r: number, g: number, b: number];

const STOPS: Record<Palette, Stop[]> = {
  inferno: [
    [0, 0, 0, 4],
    [0.13, 31, 12, 72],
    [0.25, 85, 15, 109],
    [0.38, 136, 34, 106],
    [0.5, 186, 54, 85],
    [0.63, 227, 89, 51],
    [0.75, 249, 140, 10],
    [0.88, 249, 201, 50],
    [1, 252, 255, 164],
  ],
  turbo: [
    [0, 48, 18, 59],
    [0.13, 70, 107, 227],
    [0.25, 40, 187, 236],
    [0.38, 49, 242, 153],
    [0.5, 162, 252, 60],
    [0.63, 237, 208, 58],
    [0.75, 251, 128, 34],
    [0.88, 210, 49, 5],
    [1, 122, 4, 3],
  ],
  ice: [
    [0, 4, 8, 28],
    [0.25, 22, 52, 128],
    [0.5, 34, 132, 204],
    [0.75, 126, 222, 236],
    [1, 244, 255, 255],
  ],
};

export const PALETTES: { id: Palette; label: string }[] = [
  { id: "inferno", label: "Inferno" },
  { id: "turbo", label: "Thermal" },
  { id: "ice", label: "Ice" },
];

/** A 256-entry RGB lookup table for a palette. */
export function buildLUT(palette: Palette): Uint8ClampedArray {
  const stops = STOPS[palette];
  const lut = new Uint8ClampedArray(256 * 3);
  let k = 1;
  for (let i = 0; i < 256; i++) {
    const t = i / 255;
    while (k < stops.length - 1 && t > stops[k][0]) k++;
    const a = stops[k - 1];
    const b = stops[k];
    const f = (t - a[0]) / (b[0] - a[0]);
    for (let c = 0; c < 3; c++) lut[i * 3 + c] = a[c + 1] + f * (b[c + 1] - a[c + 1]);
  }
  return lut;
}

/** The palette as a CSS gradient, for legends and swatches. */
export function paletteCSS(palette: Palette, angle = "90deg"): string {
  const stops = STOPS[palette].map(([t, r, g, b]) => `rgb(${r} ${g} ${b}) ${Math.round(t * 100)}%`);
  return `linear-gradient(${angle}, ${stops.join(", ")})`;
}

/** Trail memory in frames ↔ decay per frame: a mask weighs 1/e after `memory` frames. */
export function decayFor(memory: number): number {
  return 1 - 1 / Math.max(memory, 1);
}

/** How many frames back still matter for a trail (weights under 1 % are dropped). */
export function trailWindow(decay: number): number {
  if (decay <= 0) return 1;
  if (decay >= 1) return Number.POSITIVE_INFINITY;
  return Math.ceil(Math.log(0.01) / Math.log(decay));
}

/** Counts are checkpointed this often, so seeking back never replays a whole run. */
export const CHECKPOINT = 16;

type Masks = readonly (Uint8Array | undefined)[];

/**
 * HeatEngine holds the heat of one run at one moment (`index`) and moves it
 * to another with `seek`. Stepping forward is one pass over the pixels per
 * frame; any other move is rebuilt from the nearest checkpoint (counts) or
 * from the few frames a trail remembers.
 */
export class HeatEngine {
  readonly size: number;
  readonly trail: Float32Array;
  readonly counts: Uint16Array;
  /** The last frame folded in; −1 when empty. */
  index = -1;
  private decay = decayFor(24);
  private trailStale = false;
  /** checkpoints[k] holds the counts after k·CHECKPOINT frames (k ≥ 1). */
  private checkpoints: Uint16Array[] = [];

  constructor(
    readonly width: number,
    readonly height: number,
  ) {
    this.size = width * height;
    this.trail = new Float32Array(this.size);
    this.counts = new Uint16Array(this.size);
  }

  setDecay(decay: number) {
    if (decay !== this.decay) {
      this.decay = decay;
      this.trailStale = true;
    }
  }

  reset() {
    this.trail.fill(0);
    this.counts.fill(0);
    this.index = -1;
    this.trailStale = false;
  }

  /** Moves to frame `target`; masks[0..target] are used, missing ones count as empty. */
  seek(target: number, masks: Masks) {
    if (target < 0) {
      this.reset();
      return;
    }
    if (target === this.index && !this.trailStale) return;
    const window = trailWindow(this.decay);
    // A short hop forward only fades the frames in between; anything else
    // (a jump, a move back, a new decay) rebuilds the trail from its window.
    const hop = !this.trailStale && target > this.index && target - this.index <= window;
    const from = hop ? this.index + 1 : Number.isFinite(window) ? Math.max(0, target - window) : 0;
    this.seekCounts(target, masks);
    if (!hop) this.trail.fill(0);
    for (let f = from; f <= target; f++) this.fade(masks[f]);
    this.trailStale = false;
    this.index = target;
  }

  private fade(mask: Uint8Array | undefined) {
    const { trail, decay, size } = this;
    if (!mask) {
      for (let i = 0; i < size; i++) trail[i] *= decay;
      return;
    }
    for (let i = 0; i < size; i++) trail[i] = trail[i] * decay + mask[i];
  }

  private seekCounts(target: number, masks: Masks) {
    let from = this.index + 1;
    if (target < this.index) {
      const k = Math.min(Math.floor((target + 1) / CHECKPOINT), this.checkpoints.length);
      if (k > 0) this.counts.set(this.checkpoints[k - 1]);
      else this.counts.fill(0);
      from = k * CHECKPOINT;
    }
    const { counts, size } = this;
    for (let f = from; f <= target; f++) {
      const mask = masks[f];
      if (mask) for (let i = 0; i < size; i++) counts[i] += mask[i];
      const done = f + 1;
      if (done % CHECKPOINT === 0 && this.checkpoints.length === done / CHECKPOINT - 1) {
        this.checkpoints.push(counts.slice());
      }
    }
  }
}

/**
 * The value below which `q` of the non-zero entries lie. The heatmap is
 * scaled to this rather than to the maximum, so one flickering pixel cannot
 * dim every path in the scene.
 */
export function quantile(values: ArrayLike<number>, q: number): number {
  let max = 0;
  let n = 0;
  for (let i = 0; i < values.length; i++) {
    if (values[i] > 0) {
      n++;
      if (values[i] > max) max = values[i];
    }
  }
  if (n === 0) return 0;
  const bins = new Uint32Array(Math.ceil(max) + 1);
  for (let i = 0; i < values.length; i++) if (values[i] > 0) bins[Math.ceil(values[i])]++;
  const want = q * n;
  let seen = 0;
  for (let v = 1; v < bins.length; v++) {
    seen += bins[v];
    if (seen >= want) return v;
  }
  return max;
}

/** A separable box blur of radius r, in place (tmp is scratch of the same size). */
export function boxBlur(buf: Float32Array, tmp: Float32Array, w: number, h: number, r: number) {
  if (r <= 0) return;
  const pass = (src: Float32Array, dst: Float32Array, n: number, lines: number, step: number) => {
    const span = step === 1 ? n : 1;
    for (let l = 0; l < lines; l++) {
      const base = l * span;
      for (let p = 0; p < n; p++) {
        let sum = 0;
        let cnt = 0;
        for (let d = -r; d <= r; d++) {
          const q = p + d;
          if (q >= 0 && q < n) {
            sum += src[base + q * step];
            cnt++;
          }
        }
        dst[base + p * step] = sum / cnt;
      }
    }
  };
  pass(buf, tmp, w, h, 1); // along rows
  pass(tmp, buf, h, w, w); // along columns
}

export interface PaintOptions {
  /** The field value shown as the hottest colour. */
  peak: number;
  /** Below 1 lifts faint activity so that single passes stay visible. */
  gamma: number;
  /** 0…1 overall strength of the layer. */
  opacity: number;
  blur: number;
  /** A mask of what moves right now: the heat is thinned there, so people stay visible. */
  reveal?: Uint8Array;
  /** Nothing is underneath (thermal view): even faint heat is drawn at full strength. */
  solid?: boolean;
}

/**
 * Writes the heat field into RGBA pixels: colour from the lookup table,
 * alpha rising with heat, so quiet areas stay see-through.
 */
export function paintHeat(
  rgba: Uint8ClampedArray,
  field: ArrayLike<number>,
  scratch: [Float32Array, Float32Array],
  w: number,
  h: number,
  lut: Uint8ClampedArray,
  { peak, gamma, opacity, blur, reveal, solid }: PaintOptions,
) {
  const [t, tmp] = scratch;
  const inv = peak > 0 ? 1 / peak : 0;
  for (let i = 0; i < t.length; i++) {
    const v = field[i] * inv;
    t[i] = v >= 1 ? 1 : v;
  }
  boxBlur(t, tmp, w, h, blur);
  for (let i = 0, o = 0; i < t.length; i++, o += 4) {
    const v = t[i];
    if (v < 0.004) {
      rgba[o + 3] = 0;
      continue;
    }
    const g = v ** gamma;
    const c = Math.min(255, Math.round(g * 255)) * 3;
    rgba[o] = lut[c];
    rgba[o + 1] = lut[c + 1];
    rgba[o + 2] = lut[c + 2];
    const thin = reveal?.[i] ? 0.2 : 1;
    const alpha = solid ? g * 6 : g * 1.15 + 0.04;
    rgba[o + 3] = Math.min(1, alpha) * opacity * thin * 255;
  }
}

/** Marks the border pixels of a mask (set pixels with an unset 4-neighbour). */
export function paintOutline(
  rgba: Uint8ClampedArray,
  mask: Uint8Array | undefined,
  w: number,
  h: number,
) {
  rgba.fill(0);
  if (!mask) return;
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * w + x;
      if (!mask[i]) continue;
      const edge =
        x === 0 ||
        y === 0 ||
        x === w - 1 ||
        y === h - 1 ||
        !mask[i - 1] ||
        !mask[i + 1] ||
        !mask[i - w] ||
        !mask[i + w];
      if (!edge) continue;
      const o = i * 4;
      rgba[o] = 255;
      rgba[o + 1] = 255;
      rgba[o + 2] = 255;
      rgba[o + 3] = 235;
    }
  }
}
