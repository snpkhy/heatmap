import { useColorScheme } from "@mui/material/styles";

// Chart series colours. The three stay distinguishable for colour-blind
// readers in every pairing, so charts use at most three series. Dark mode
// has its own steps for the dark surface.
const LIGHT = ["#c9500f", "#2a78d6", "#1b9e6f"];
const DARK = ["#ff9447", "#5aa2f0", "#4fcf9a"];

export function useIsDark(): boolean {
  const { mode, systemMode } = useColorScheme();
  // Before the scheme is known (first paint) the app is dark, like its default.
  return (mode === "system" ? systemMode : mode) !== "light";
}

export function useSeriesColors(): string[] {
  return useIsDark() ? DARK : LIGHT;
}
