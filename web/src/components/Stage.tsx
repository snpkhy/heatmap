import Box from "@mui/material/Box";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  buildLUT,
  decayFor,
  type HeatEngine,
  type Palette,
  paintHeat,
  paintOutline,
} from "../heat/engine";
import type { FrameStore } from "../heat/frames";

/** What the stage shows: fading trails, the heat built up so far, or plain footage. */
export type LayerMode = "trails" | "heatmap" | "footage";

export interface Layers {
  mode: LayerMode;
  palette: Palette;
  /** 0…1 strength of the heat layer. */
  opacity: number;
  /** How many frames a trail takes to fade. */
  memory: number;
  /** Show the footage under the heat (off: heat on black, like a thermal camera). */
  footage: boolean;
  /** Draw a line around everything that moves in the current frame. */
  outline: boolean;
}

export const STAGE_BLACK = "#07060a";

/**
 * The trail value drawn as hottest. One person crossing a pixel leaves about
 * two thirds of it, so a single trail glows orange and fades through purple,
 * and only places where trails overlap turn white-hot.
 */
export function trailPeak(memory: number): number {
  return 1.1 * Math.min(memory, 10);
}

interface Buffers {
  canvas: HTMLCanvasElement;
  ctx: CanvasRenderingContext2D;
  pixels: ImageData;
  scratch: [Float32Array, Float32Array];
  blur: number;
}

function makeBuffers(w: number, h: number): Buffers | null {
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  return {
    canvas,
    ctx,
    pixels: ctx.createImageData(w, h),
    scratch: [new Float32Array(w * h), new Float32Array(w * h)],
    blur: Math.max(1, Math.round(w / 160)),
  };
}

function draw(
  ctx: CanvasRenderingContext2D,
  W: number,
  H: number,
  store: FrameStore,
  engine: HeatEngine,
  index: number,
  layers: Layers,
  lut: Uint8ClampedArray,
  buf: Buffers,
) {
  ctx.globalCompositeOperation = "source-over";
  ctx.globalAlpha = 1;
  ctx.fillStyle = STAGE_BLACK;
  ctx.fillRect(0, 0, W, H);
  if (index < 0 || index >= store.ready) return;

  const heat = layers.mode !== "footage";
  const img = store.images[index];
  if (img && (layers.footage || !heat)) {
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(img, 0, 0, W, H);
    if (heat) {
      // Dim the footage so the heat reads as light on top of it.
      ctx.fillStyle = "rgba(7, 6, 10, 0.36)";
      ctx.fillRect(0, 0, W, H);
    }
  }

  const { gridW, gridH } = store;
  if (heat) {
    const trails = layers.mode === "trails";
    engine.setDecay(decayFor(layers.memory));
    engine.seek(index, store.masks);
    paintHeat(
      buf.pixels.data,
      trails ? engine.trail : engine.counts,
      buf.scratch,
      gridW,
      gridH,
      lut,
      {
        peak: trails ? trailPeak(layers.memory) : store.peak,
        // Without footage the palette's dark end is all there is, so lift it.
        gamma: (trails ? 1 : 0.5) * (layers.footage ? 1 : 0.75),
        opacity: layers.opacity,
        blur: buf.blur,
        reveal: layers.footage ? store.masks[index] : undefined,
        solid: !layers.footage,
      },
    );
    buf.ctx.putImageData(buf.pixels, 0, 0);
    ctx.drawImage(buf.canvas, 0, 0, W, H);
    // A second, additive pass makes the hottest places glow.
    ctx.globalCompositeOperation = "lighter";
    ctx.globalAlpha = 0.16;
    ctx.drawImage(buf.canvas, 0, 0, W, H);
    ctx.globalCompositeOperation = "source-over";
    ctx.globalAlpha = 1;
  }
  if (layers.outline) {
    paintOutline(buf.pixels.data, store.masks[index], gridW, gridH);
    buf.ctx.putImageData(buf.pixels, 0, 0);
    ctx.drawImage(buf.canvas, 0, 0, W, H);
  }
}

interface Props {
  store: FrameStore;
  engine: HeatEngine;
  /** The frame to show (position within the run). */
  index: number;
  layers: Layers;
  /** Changes whenever the store has loaded more frames. */
  version: number;
  width: number;
  height: number;
  label: string;
  children?: ReactNode;
}

/** The picture: one canvas with the footage, the heat layer and the outline. */
export function Stage({
  store,
  engine,
  index,
  layers,
  version,
  width,
  height,
  label,
  children,
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [resized, setResized] = useState(0);
  const buffers = useMemo(() => makeBuffers(store.gridW, store.gridH), [store]);
  const lut = useMemo(() => buildLUT(layers.palette), [layers.palette]);

  // Keep the canvas as sharp as the screen, whatever size the layout gives it.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const resize = () => {
      const ratio = Math.min(window.devicePixelRatio || 1, 2);
      const w = Math.max(1, Math.round(canvas.clientWidth * ratio));
      const h = Math.max(1, Math.round(canvas.clientHeight * ratio));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
        setResized((n) => n + 1);
      }
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    return () => observer.disconnect();
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: version and resized only say "draw again".
  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx || !buffers) return;
    draw(ctx, canvas.width, canvas.height, store, engine, index, layers, lut, buffers);
  }, [store, engine, index, layers, lut, buffers, version, resized]);

  return (
    <Box
      sx={{
        position: "relative",
        display: "flex",
        justifyContent: "center",
        borderRadius: "20px",
        overflow: "hidden",
        bgcolor: STAGE_BLACK,
        boxShadow:
          "0 0 0 1px rgba(255, 244, 232, 0.08), 0 30px 80px -40px rgba(249, 140, 10, 0.35)",
      }}
    >
      <Box
        component="canvas"
        ref={canvasRef}
        role="img"
        aria-label={label}
        sx={{
          display: "block",
          // As wide as the column allows, but short enough that the controls
          // under it stay on the screen.
          width: `min(100%, calc(max(300px, 100vh - 330px) * ${width} / ${height}))`,
          aspectRatio: `${width} / ${height}`,
        }}
      />
      {children}
    </Box>
  );
}
