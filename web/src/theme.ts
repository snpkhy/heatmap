import { createTheme } from "@mui/material/styles";

// App-wide MUI theme. Light and dark schemes follow the OS setting by default
// and can be switched with useColorScheme().
export const theme = createTheme({
  colorSchemes: {
    light: true,
    dark: true,
  },
  cssVariables: {
    colorSchemeSelector: "class",
  },
  shape: {
    borderRadius: 10,
  },
});
