import Box from "@mui/material/Box";
import Slider from "@mui/material/Slider";
import { useId, useMemo } from "react";

interface Props {
  /** Share of the frame that moves, per frame. */
  activity: Float32Array;
  /** Frames [0, ready) are loaded. */
  ready: number;
  index: number;
  onSeek: (index: number) => void;
  /** The accessible name of the scrubber. */
  label: string;
  disabled?: boolean;
}

/**
 * The scrubber. Behind the playhead it draws how much was moving in every
 * frame, so busy moments can be found at a glance.
 */
export function Timeline({ activity, ready, index, onSeek, label, disabled }: Props) {
  const gradient = useId();
  const last = Math.max(activity.length - 1, 1);
  const area = useMemo(() => {
    if (ready === 0) return "";
    // Scale to a high quantile, not the maximum: some detectors flag most of
    // the picture in their first frames, and that spike would flatten the rest.
    const sorted = Array.from(activity.subarray(0, ready)).sort((x, y) => x - y);
    const top = sorted[Math.floor(0.97 * (ready - 1))] || sorted[ready - 1];
    const scale = top > 0 ? 80 / top : 0;
    let d = "M0,100";
    for (let i = 0; i < ready; i++) {
      d += `L${i},${Math.max(100 - activity[i] * scale, 2).toFixed(1)}`;
    }
    return `${d}L${ready - 1},100Z`;
  }, [activity, ready]);

  return (
    <Box sx={{ position: "relative", flexGrow: 1, height: 48, minWidth: 0 }}>
      <Box
        component="svg"
        viewBox={`0 0 ${last} 100`}
        preserveAspectRatio="none"
        aria-hidden
        sx={{
          position: "absolute",
          inset: 0,
          width: "100%",
          height: "100%",
          borderRadius: "8px",
          bgcolor: "action.hover",
        }}
      >
        <defs>
          <linearGradient id={gradient} x1="0" y1="1" x2="0" y2="0">
            <stop offset="0" stopColor="#ba3655" stopOpacity="0.55" />
            <stop offset="0.6" stopColor="#f98c0a" stopOpacity="0.9" />
            <stop offset="1" stopColor="#fcffa4" />
          </linearGradient>
        </defs>
        {area && <path d={area} fill={`url(#${gradient})`} />}
        {/* What has not been played yet is veiled with the page colour. */}
        <Box
          component="rect"
          x={index}
          y="0"
          width={Math.max(last - index, 0)}
          height="100"
          sx={{ fill: "var(--mui-palette-background-default)", opacity: 0.5 }}
        />
      </Box>
      <Slider
        aria-label={label}
        value={index}
        min={0}
        max={activity.length - 1}
        disabled={disabled}
        onChange={(_, v) => onSeek(v as number)}
        sx={{
          position: "absolute",
          inset: 0,
          height: "100%",
          p: "0 !important",
          borderRadius: 0,
          "& .MuiSlider-rail, & .MuiSlider-track": { opacity: 0 },
          "& .MuiSlider-thumb": {
            width: 3,
            height: "100%",
            borderRadius: "2px",
            bgcolor: "#fff",
            boxShadow: "0 0 10px rgba(255,255,255,0.7)",
            transition: "none",
            "&::before, &::after": { display: "none" },
            "&:hover, &.Mui-focusVisible, &.Mui-active": {
              boxShadow: "0 0 0 6px rgba(255,255,255,0.16), 0 0 10px rgba(255,255,255,0.7)",
            },
          },
        }}
      />
    </Box>
  );
}
