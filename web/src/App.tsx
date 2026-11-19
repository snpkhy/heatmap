import DarkModeOutlinedIcon from "@mui/icons-material/DarkModeOutlined";
import LightModeOutlinedIcon from "@mui/icons-material/LightModeOutlined";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import IconButton from "@mui/material/IconButton";
import { useColorScheme } from "@mui/material/styles";
import Tooltip from "@mui/material/Tooltip";
import { useEffect } from "react";
import { Brand } from "./components/Brand";
import { LANGUAGES, useLanguage } from "./i18n";
import { Compare } from "./pages/Compare";
import { History } from "./pages/History";
import { Scenes } from "./pages/Scenes";
import { Viewer } from "./pages/Viewer";
import { useIsDark } from "./palette";
import { type Route, routeHref, useRoute } from "./router";

const links: { key: "scenes" | "history"; route: Route; pages: Route["page"][] }[] = [
  { key: "scenes", route: { page: "launch" }, pages: ["launch"] },
  { key: "history", route: { page: "runs" }, pages: ["runs", "compare"] },
];

// App is the root layout: the top bar and the current page.
export function App() {
  const { setMode } = useColorScheme();
  const isDark = useIsDark();
  const { lang, setLang, t } = useLanguage();
  const route = useRoute();
  const place = route.page === "run" ? route.id : route.page;

  // A new page starts at its top, like a page load would.
  // biome-ignore lint/correctness/useExhaustiveDependencies: place is the trigger.
  useEffect(() => {
    window.scrollTo?.(0, 0);
  }, [place]);

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default", color: "text.primary" }}>
      <Box
        component="header"
        sx={{
          position: "sticky",
          top: 0,
          zIndex: 10,
          borderBottom: 1,
          borderColor: "divider",
          backdropFilter: "blur(14px)",
          bgcolor: "color-mix(in srgb, var(--mui-palette-background-default) 78%, transparent)",
        }}
      >
        <Container
          maxWidth="xl"
          sx={{ display: "flex", alignItems: "center", gap: { xs: 2, sm: 4 }, height: 64 }}
        >
          <Brand />
          <Box
            component="nav"
            aria-label={t.nav.main}
            sx={{ display: "flex", gap: 0.5, flexGrow: 1 }}
          >
            {links.map((l) => {
              const current = l.pages.includes(route.page);
              return (
                <Box
                  key={l.key}
                  component="a"
                  href={routeHref(l.route)}
                  aria-current={current ? "page" : undefined}
                  sx={{
                    px: 1.5,
                    py: 0.75,
                    borderRadius: "999px",
                    fontSize: 14,
                    fontWeight: 600,
                    textDecoration: "none",
                    color: current ? "text.primary" : "text.secondary",
                    bgcolor: current ? "action.selected" : "transparent",
                    transition: "color .15s, background-color .15s",
                    "&:hover": { color: "text.primary" },
                  }}
                >
                  {t.nav[l.key]}
                </Box>
              );
            })}
          </Box>
          <Box
            role="group"
            aria-label={t.nav.language}
            sx={{
              display: "flex",
              p: "3px",
              borderRadius: "999px",
              border: 1,
              borderColor: "divider",
            }}
          >
            {LANGUAGES.map((l) => (
              <Box
                key={l.id}
                component="button"
                type="button"
                lang={l.id}
                title={l.name}
                aria-label={l.name}
                aria-pressed={lang === l.id}
                onClick={() => setLang(l.id)}
                sx={{
                  px: 1.25,
                  py: 0.5,
                  border: 0,
                  borderRadius: "999px",
                  cursor: "pointer",
                  font: "inherit",
                  fontSize: 12,
                  fontWeight: 700,
                  letterSpacing: "0.04em",
                  color: lang === l.id ? "text.primary" : "text.secondary",
                  bgcolor: lang === l.id ? "action.selected" : "transparent",
                  transition: "color .15s, background-color .15s",
                  "&:hover": { color: "text.primary" },
                }}
              >
                {l.label}
              </Box>
            ))}
          </Box>
          <Tooltip title={isDark ? t.nav.lightTheme : t.nav.darkTheme}>
            <IconButton
              aria-label={t.nav.toggleScheme}
              onClick={() => setMode(isDark ? "light" : "dark")}
            >
              {isDark ? <LightModeOutlinedIcon /> : <DarkModeOutlinedIcon />}
            </IconButton>
          </Tooltip>
        </Container>
      </Box>
      <Container maxWidth="xl" component="main" sx={{ py: { xs: 3, md: 4 } }}>
        {route.page === "launch" && <Scenes />}
        {route.page === "runs" && <History />}
        {route.page === "run" && <Viewer key={route.id} id={route.id} />}
        {route.page === "compare" && <Compare ids={route.ids} />}
      </Container>
    </Box>
  );
}
