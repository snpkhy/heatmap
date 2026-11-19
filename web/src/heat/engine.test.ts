import { describe, expect, it } from "vitest";
import {
  boxBlur,
  buildLUT,
  CHECKPOINT,
  decayFor,
  HeatEngine,
  PALETTES,
  paintHeat,
  paintOutline,
  paletteCSS,
  quantile,
  trailWindow,
} from "./engine";
import { gridSize } from "./frames";

/** n masks of w×h where frame f lights the pixel f % (w·h). */
function walk(n: number, size: number): Uint8Array[] {
  return Array.from({ length: n }, (_, f) => {
    const m = new Uint8Array(size);
    m[f % size] = 1;
    return m;
  });
}

function fresh(target: number, masks: Uint8Array[], decay: number): HeatEngine {
  const e = new HeatEngine(4, 2);
  e.setDecay(decay);
  e.seek(target, masks);
  return e;
}

describe("HeatEngine", () => {
  const masks = walk(60, 8);

  it("counts how often a pixel moved and lets trails fade", () => {
    const e = new HeatEngine(4, 2);
    e.setDecay(0.5);
    e.seek(0, masks);
    expect(Array.from(e.counts)).toEqual([1, 0, 0, 0, 0, 0, 0, 0]);
    expect(e.trail[0]).toBe(1);
    e.seek(1, masks);
    expect(e.trail[0]).toBe(0.5);
    expect(e.trail[1]).toBe(1);
    e.seek(9, masks);
    expect(e.counts[0]).toBe(2); // frames 0 and 8
    expect(e.counts[1]).toBe(2);
    expect(e.counts[2]).toBe(1);
    expect(e.index).toBe(9);
  });

  it("gives the same heat however a frame is reached", () => {
    const decay = decayFor(6);
    const e = new HeatEngine(4, 2);
    e.setDecay(decay);
    // Play forward, hop, jump back before and after checkpoints, jump far ahead.
    for (const target of [0, 1, 2, 5, 40, CHECKPOINT - 1, CHECKPOINT, 3, 59, 33, 34]) {
      e.seek(target, masks);
      const want = fresh(target, masks, decay);
      expect(Array.from(e.counts), `counts at ${target}`).toEqual(Array.from(want.counts));
      // Trails drop weights under 1 %, so two routes may differ by about that much.
      for (let i = 0; i < e.size; i++) {
        expect(Math.abs(e.trail[i] - want.trail[i]), `trail at ${target}`).toBeLessThan(0.02);
      }
    }
  });

  it("rebuilds the trail when the decay changes and treats missing masks as empty", () => {
    const e = new HeatEngine(4, 2);
    e.setDecay(0.5);
    e.seek(2, masks);
    expect(e.trail[0]).toBe(0.25);
    e.setDecay(1);
    e.seek(2, masks);
    expect(e.trail[0]).toBe(1);

    const gaps = [masks[0], undefined, masks[2]];
    const g = new HeatEngine(4, 2);
    g.setDecay(0.5);
    g.seek(2, gaps);
    expect(Array.from(g.counts.slice(0, 3))).toEqual([1, 0, 1]);
    expect(g.trail[0]).toBe(0.25);

    g.seek(-1, gaps);
    expect(g.index).toBe(-1);
    expect(g.counts[0]).toBe(0);
  });
});

describe("heat helpers", () => {
  it("relates trail memory, decay and the window that still matters", () => {
    expect(decayFor(10)).toBeCloseTo(0.9);
    expect(decayFor(0)).toBe(0);
    expect(trailWindow(0.9)).toBe(44);
    expect(trailWindow(1)).toBe(Number.POSITIVE_INFINITY);
    expect(trailWindow(0)).toBe(1);
  });

  it("builds palettes that run from dark to bright", () => {
    for (const { id } of PALETTES) {
      const lut = buildLUT(id);
      expect(lut).toHaveLength(768);
      const first = lut[0] + lut[1] + lut[2];
      const mid = lut[384] + lut[385] + lut[386];
      expect(first).toBeLessThan(mid);
      expect(paletteCSS(id)).toContain("linear-gradient(90deg");
    }
    const inferno = buildLUT("inferno");
    expect(Array.from(inferno.slice(0, 3))).toEqual([0, 0, 4]);
    expect(Array.from(inferno.slice(765))).toEqual([252, 255, 164]);
  });

  it("finds a quantile of the non-zero values", () => {
    expect(quantile(new Uint16Array(10), 0.99)).toBe(0);
    const v = new Uint16Array(1000);
    for (let i = 0; i < 200; i++) v[i] = 3;
    v[999] = 500; // one flickering pixel
    expect(quantile(v, 0.99)).toBe(3);
    expect(quantile(v, 1)).toBe(500);
  });

  it("blurs without changing a flat field", () => {
    const flat = new Float32Array(12).fill(0.5);
    boxBlur(flat, new Float32Array(12), 4, 3, 1);
    for (const v of flat) expect(v).toBeCloseTo(0.5);

    const spot = new Float32Array(25);
    spot[12] = 9;
    boxBlur(spot, new Float32Array(25), 5, 5, 1);
    // The spot spreads evenly over the 3×3 block around it and no further.
    spot.forEach((v, i) => {
      const inner = i % 5 >= 1 && i % 5 <= 3 && i >= 5 && i < 20;
      expect(v).toBeCloseTo(inner ? 1 : 0);
    });
  });

  it("paints heat as colour with alpha, and keeps moving things visible", () => {
    const lut = buildLUT("inferno");
    const field = new Float32Array([0, 2, 4, 4]);
    const rgba = new Uint8ClampedArray(16);
    const scratch: [Float32Array, Float32Array] = [new Float32Array(4), new Float32Array(4)];
    const reveal = new Uint8Array([0, 0, 0, 1]);
    paintHeat(rgba, field, scratch, 4, 1, lut, { peak: 4, gamma: 1, opacity: 1, blur: 0, reveal });
    expect(rgba[3]).toBe(0); // no heat: see-through
    expect(rgba[7]).toBeGreaterThan(100);
    expect(rgba[11]).toBe(255); // the hottest colour, opaque
    expect(Array.from(rgba.slice(8, 11))).toEqual([252, 255, 164]);
    expect(rgba[15]).toBeLessThan(rgba[11] / 2); // thinned where something moves now

    // With nothing underneath, faint heat is drawn at full strength too.
    const faint = new Float32Array([0, 1, 4, 4]);
    const base = { peak: 4, gamma: 1, opacity: 1, blur: 0 };
    paintHeat(rgba, faint, scratch, 4, 1, lut, base);
    expect(rgba[7]).toBeLessThan(128);
    paintHeat(rgba, faint, scratch, 4, 1, lut, { ...base, solid: true });
    expect(rgba[7]).toBe(255);

    paintHeat(rgba, field, scratch, 4, 1, lut, { peak: 0, gamma: 1, opacity: 1, blur: 0 });
    expect(rgba[11]).toBe(0);
  });

  it("outlines a mask", () => {
    const mask = new Uint8Array(25);
    for (const i of [6, 7, 8, 11, 12, 13, 16, 17, 18]) mask[i] = 1; // 3×3 block in 5×5
    const rgba = new Uint8ClampedArray(100);
    paintOutline(rgba, mask, 5, 5);
    expect(rgba[6 * 4 + 3]).toBeGreaterThan(0); // a corner of the block
    expect(rgba[12 * 4 + 3]).toBe(0); // its centre
    expect(rgba[0 + 3]).toBe(0); // outside
    paintOutline(rgba, undefined, 5, 5);
    expect(rgba.some((v) => v !== 0)).toBe(false);
  });

  it("sizes the heat grid", () => {
    expect(gridSize(160, 120)).toEqual([160, 120]);
    expect(gridSize(380, 244)).toEqual([380, 244]);
    expect(gridSize(768, 576)).toEqual([384, 288]);
  });
});
