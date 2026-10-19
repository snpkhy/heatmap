import DarkModeIcon from "@mui/icons-material/DarkMode";
import LightModeIcon from "@mui/icons-material/LightMode";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import IconButton from "@mui/material/IconButton";
import { useColorScheme } from "@mui/material/styles";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";

// App is the root layout. Screens (run launcher, viewer, metrics, comparison)
// are added once the Go API exists — see docs/architecture.md.
export function App() {
  const { mode, systemMode, setMode } = useColorScheme();
  const isDark = (mode === "system" ? systemMode : mode) === "dark";

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }}>
      <AppBar position="static" elevation={0}>
        <Toolbar>
          <Typography variant="h6" component="h1" sx={{ flexGrow: 1 }}>
            heatmap
          </Typography>
          <IconButton
            color="inherit"
            aria-label="toggle color scheme"
            onClick={() => setMode(isDark ? "light" : "dark")}
          >
            {isDark ? <LightModeIcon /> : <DarkModeIcon />}
          </IconButton>
        </Toolbar>
      </AppBar>
      <Container sx={{ py: 4 }}>
        <Typography color="text.secondary">
          Motion heatmap UI. Screens will appear here once the pipeline API is ready.
        </Typography>
      </Container>
    </Box>
  );
}
