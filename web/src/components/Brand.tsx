import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { routeHref } from "../router";
import { fonts } from "../theme";

/** The heat spot: the brightest point of a heatmap, as a logo. */
export function HeatSpot({ size = 26 }: { size?: number }) {
  return (
    <Box
      aria-hidden
      sx={{
        width: size,
        height: size,
        borderRadius: "50%",
        flex: "none",
        background:
          "radial-gradient(circle at 50% 50%, #fcffa4 0%, #f98c0a 34%, #ba3655 62%, #55106d 84%, transparent 100%)",
        boxShadow: "0 0 18px rgba(249, 140, 10, 0.45)",
      }}
    />
  );
}

export function Brand() {
  return (
    <Box
      component="a"
      href={routeHref({ page: "launch" })}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.25,
        color: "text.primary",
        textDecoration: "none",
      }}
    >
      <HeatSpot />
      <Typography
        component="h1"
        sx={{
          fontFamily: fonts.display,
          fontSize: 21,
          fontWeight: 700,
          lineHeight: 1,
          letterSpacing: "-0.03em",
        }}
      >
        Heatmap
      </Typography>
    </Box>
  );
}
