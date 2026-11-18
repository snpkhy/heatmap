import DarkModeIcon from "@mui/icons-material/DarkMode";
import LightModeIcon from "@mui/icons-material/LightMode";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import IconButton from "@mui/material/IconButton";
import { useColorScheme } from "@mui/material/styles";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import { Compare } from "./pages/Compare";
import { Launcher } from "./pages/Launcher";
import { RunPage } from "./pages/RunPage";
import { RunsPage } from "./pages/RunsPage";
import { type Route, routeHref, useRoute } from "./router";

const tabs: { label: string; route: Route }[] = [
  { label: "New run", route: { page: "launch" } },
  { label: "Runs", route: { page: "runs" } },
  { label: "Compare", route: { page: "compare", ids: [] } },
];

function tabIndex(route: Route): number {
  if (route.page === "run") return 1;
  return Math.max(
    0,
    tabs.findIndex((t) => t.route.page === route.page),
  );
}

// App is the root layout: navigation, theme toggle and the current page.
export function App() {
  const { mode, systemMode, setMode } = useColorScheme();
  const isDark = (mode === "system" ? systemMode : mode) === "dark";
  const route = useRoute();

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }}>
      <AppBar position="static" elevation={0}>
        <Toolbar>
          <Typography variant="h6" component="h1" sx={{ mr: 4 }}>
            heatmap
          </Typography>
          <Tabs
            value={tabIndex(route)}
            textColor="inherit"
            indicatorColor="secondary"
            aria-label="navigation"
            sx={{ flexGrow: 1 }}
          >
            {tabs.map((t) => (
              <Tab key={t.label} label={t.label} component="a" href={routeHref(t.route)} />
            ))}
          </Tabs>
          <IconButton
            color="inherit"
            aria-label="toggle color scheme"
            onClick={() => setMode(isDark ? "light" : "dark")}
          >
            {isDark ? <LightModeIcon /> : <DarkModeIcon />}
          </IconButton>
        </Toolbar>
      </AppBar>
      <Container maxWidth="xl" sx={{ py: 4 }}>
        {route.page === "launch" && <Launcher />}
        {route.page === "runs" && <RunsPage />}
        {route.page === "run" && <RunPage key={route.id} id={route.id} />}
        {route.page === "compare" && <Compare ids={route.ids} />}
      </Container>
    </Box>
  );
}
