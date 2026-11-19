import type { NoiseSpec, SequenceInfo } from "./api";

/** A metric in [0, 1] with three decimals, or an em dash when unknown. */
export function metric(v: number | null | undefined): string {
  return v === null || v === undefined || Number.isNaN(v) ? "—" : v.toFixed(3);
}

export function fps(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return v >= 100 ? v.toFixed(0) : v.toFixed(1);
}

export function decibels(v: number | null | undefined, unit = "dB"): string {
  return v === null || v === undefined || Number.isNaN(v) ? "—" : `${v.toFixed(1)} ${unit}`;
}

/** A share in [0, 1] as a percentage: one decimal under 10 %, none above. */
export function percent(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  const p = v * 100;
  return `${p < 10 ? p.toFixed(1) : p.toFixed(0)}%`;
}

/** The words noiseLabel needs; the defaults are English. */
export interface NoiseWords {
  none: string;
  gaussian: string;
  saltPepper: string;
}

const noiseWords: NoiseWords = { none: "none", gaussian: "gaussian", saltPepper: "salt & pepper" };

/** A noise setting in words; the result equals `words.none` when there is no noise. */
export function noiseLabel(n: NoiseSpec | undefined | string, words = noiseWords): string {
  if (n === undefined) return words.none;
  if (typeof n === "string") return n && n !== "none" ? n : words.none;
  if (!n.type || n.type === "none" || n.level === 0) return words.none;
  const kind =
    n.type === "salt_pepper" ? words.saltPepper : n.type === "gaussian" ? words.gaussian : n.type;
  return `${kind} ${n.level}`;
}

export function dateTime(iso: string, locale?: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(locale);
}

const compassNames = [
  "→ right",
  "↗ up-right",
  "↑ up",
  "↖ up-left",
  "← left",
  "↙ down-left",
  "↓ down",
  "↘ down-right",
];

/** Compass name of a screen angle in degrees (0 = right, 90 = up), from eight names. */
export function compass(deg: number, names = compassNames): string {
  return names[Math.round((((deg % 360) + 360) % 360) / 45) % 8];
}

/** Human name of a processing step key such as "2-running_avg_otsu". */
export function stageLabel(key: string): string {
  const [, num, name] = /^(\d+)-(.*)$/.exec(key) ?? [];
  return num ? `${num}. ${name.replaceAll("_", " ")}` : key;
}

/**
 * A readable title for a scene id or name: "cdnet/shadow/peopleInShade" →
 * "People in shade", "tramCrossroad_1fps" → "Tram crossroad 1fps". Words in
 * capitals (PETS2006) are kept as they are.
 */
export function sceneTitle(idOrName: string): string {
  const name = idOrName.split("/").pop() ?? idOrName;
  const words = name
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .split(" ")
    .filter(Boolean)
    .map((w) => (/^[A-Z0-9]+$/.test(w) && w.length > 1 ? w : w.toLowerCase()));
  const title = words.join(" ");
  return title.charAt(0).toUpperCase() + title.slice(1);
}

/** Where a scene comes from, in the words of the gallery. */
export function sceneSource(seq: Pick<SequenceInfo, "source" | "category">): string {
  if (seq.source === "synthetic") return "Built-in";
  if (seq.source === "frames") return "Your footage";
  return `CDnet · ${sceneTitle(seq.category).toLowerCase()}`;
}
