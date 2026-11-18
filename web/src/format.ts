import type { NoiseSpec } from "./api";

/** A metric in [0, 1] with three decimals, or an em dash when unknown. */
export function metric(v: number | null | undefined): string {
  return v === null || v === undefined || Number.isNaN(v) ? "—" : v.toFixed(3);
}

export function fps(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return v >= 100 ? v.toFixed(0) : v.toFixed(1);
}

export function decibels(v: number | null | undefined): string {
  return v === null || v === undefined || Number.isNaN(v) ? "—" : `${v.toFixed(1)} dB`;
}

export function noiseLabel(n: NoiseSpec | undefined | string): string {
  if (n === undefined) return "none";
  if (typeof n === "string") return n || "none";
  if (!n.type || n.type === "none" || n.level === 0) return "none";
  return `${n.type === "salt_pepper" ? "salt & pepper" : n.type} ${n.level}`;
}

export function dateTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString();
}

/** Compass name of a screen angle in degrees (0 = right, 90 = up). */
export function compass(deg: number): string {
  const names = [
    "→ right",
    "↗ up-right",
    "↑ up",
    "↖ up-left",
    "← left",
    "↙ down-left",
    "↓ down",
    "↘ down-right",
  ];
  return names[Math.round((((deg % 360) + 360) % 360) / 45) % 8];
}

/** Human name of a stage key such as "2-running_avg_otsu". */
export function stageLabel(key: string): string {
  const [, num, name] = /^(\d+)-(.*)$/.exec(key) ?? [];
  return num ? `${num}. ${name.replaceAll("_", " ")}` : key;
}
