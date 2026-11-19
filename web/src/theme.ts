import { createTheme } from "@mui/material/styles";

// The look of the app. Dark is the default: the heat layer reads best on a
// dark surface, the way a thermal camera shows it. The accent is the warm
// end of that same heat scale.

export const fonts = {
  body: '"Inter Variable", system-ui, -apple-system, "Segoe UI", sans-serif',
  // Set per language in the baseline styles below: Space Grotesk has no Cyrillic.
  display: "var(--font-display)",
  mono: '"JetBrains Mono Variable", ui-monospace, "SF Mono", Menlo, monospace',
};

export const theme = createTheme({
  cssVariables: { colorSchemeSelector: "class" },
  defaultColorScheme: "dark",
  colorSchemes: {
    dark: {
      palette: {
        primary: { main: "#ff9447", contrastText: "#1a0f06" },
        secondary: { main: "#f2c45a" },
        success: { main: "#58c98b" },
        error: { main: "#ff6b61" },
        warning: { main: "#f2b23e" },
        background: { default: "#0c0a0e", paper: "#151218" },
        text: { primary: "#f5f0ea", secondary: "#a69fa6" },
        divider: "rgba(255, 244, 232, 0.09)",
      },
    },
    light: {
      palette: {
        primary: { main: "#c9500f", contrastText: "#ffffff" },
        secondary: { main: "#8a5a00" },
        success: { main: "#1f8a55" },
        background: { default: "#f5f1ec", paper: "#fffdfa" },
        text: { primary: "#1c1714", secondary: "#6b625c" },
        divider: "rgba(40, 24, 10, 0.12)",
      },
    },
  },
  shape: { borderRadius: 14 },
  typography: {
    fontFamily: fonts.body,
    h1: { fontFamily: fonts.display, fontWeight: 600, letterSpacing: "-0.04em", lineHeight: 1 },
    h2: { fontFamily: fonts.display, fontWeight: 600, letterSpacing: "-0.035em", lineHeight: 1.05 },
    h3: { fontFamily: fonts.display, fontWeight: 600, letterSpacing: "-0.025em", lineHeight: 1.15 },
    h6: { fontWeight: 600, letterSpacing: "-0.01em" },
    subtitle1: { fontWeight: 600, letterSpacing: "-0.005em" },
    subtitle2: { fontWeight: 600 },
    button: { textTransform: "none", fontWeight: 600, letterSpacing: 0 },
    overline: { fontWeight: 600, letterSpacing: "0.14em", fontSize: "0.7rem", lineHeight: 1.6 },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        ":root": {
          "--font-display": '"Space Grotesk Variable", "Inter Variable", system-ui, sans-serif',
        },
        // Ukrainian headings use a grotesque that has Cyrillic, for letters and digits alike.
        ':root[lang="uk"]': {
          "--font-display": '"Geologica Variable", "Inter Variable", system-ui, sans-serif',
        },
        body: { fontFeatureSettings: '"cv11", "ss01"', WebkitFontSmoothing: "antialiased" },
        "::selection": { background: "rgba(255, 148, 71, 0.35)" },
      },
    },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: { root: { borderRadius: 999, paddingInline: 18 } },
    },
    MuiPaper: { styleOverrides: { root: { backgroundImage: "none" } } },
    MuiTooltip: {
      defaultProps: { arrow: true },
      styleOverrides: { tooltip: { fontSize: "0.75rem", fontWeight: 500 } },
    },
    MuiTableCell: {
      styleOverrides: {
        head: { fontSize: "0.72rem", letterSpacing: "0.08em", textTransform: "uppercase" },
      },
    },
    MuiTab: { styleOverrides: { root: { minHeight: 44, fontWeight: 600 } } },
    MuiToggleButton: {
      styleOverrides: { root: { textTransform: "none", fontWeight: 600, paddingInline: 14 } },
    },
  },
});
