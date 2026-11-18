import { useColorScheme } from "@mui/material/styles";

// Categorical slots 1–3 of the validated chart palette. They stay
// distinguishable for colour-blind readers in every pairing, so charts use
// at most three series. Dark mode has its own steps for the dark surface.
const LIGHT = ["#2a78d6", "#eb6834", "#1baf7a"];
const DARK = ["#3987e5", "#d95926", "#199e70"];

export function useSeriesColors(): string[] {
  const { mode, systemMode } = useColorScheme();
  return (mode === "system" ? systemMode : mode) === "dark" ? DARK : LIGHT;
}

/** Chip colors per method category, from MUI's semantic palette. */
export const categoryColor: Record<
  string,
  "default" | "primary" | "secondary" | "success" | "warning" | "info" | "error"
> = {
  filtering: "info",
  fourier: "info",
  color: "secondary",
  histogram: "secondary",
  segmentation: "primary",
  morphology: "success",
  edges: "warning",
  texture: "warning",
  stats: "default",
  keypoints: "warning",
  geometry: "success",
};
