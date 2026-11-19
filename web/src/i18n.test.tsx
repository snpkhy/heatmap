import { ThemeProvider } from "@mui/material/styles";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { initialLang, LanguageProvider, messages } from "./i18n";
import { Compare, runLabel } from "./pages/Compare";
import { Viewer } from "./pages/Viewer";
import { listItem, mockFetch, pipelines, run, sequences, summary } from "./test/helpers";
import { theme } from "./theme";

function renderIn(lang: "en" | "uk", ui: React.ReactNode) {
  localStorage.setItem("heatmap.lang", lang);
  return render(
    <ThemeProvider theme={theme} defaultMode="dark">
      <LanguageProvider>{ui}</LanguageProvider>
    </ThemeProvider>,
  );
}

describe("language", () => {
  it("starts from the saved choice, else from the browser", () => {
    expect(initialLang()).toBe("en"); // jsdom speaks en-US
    localStorage.setItem("heatmap.lang", "uk");
    expect(initialLang()).toBe("uk");
    localStorage.setItem("heatmap.lang", "xx");
    expect(initialLang()).toBe("en");
  });

  it("declines Ukrainian numbers", () => {
    const { frames, viewer } = messages.uk;
    expect([1, 2, 5, 11, 21, 22, 112, 300].map(frames)).toEqual([
      "1 кадр",
      "2 кадри",
      "5 кадрів",
      "11 кадрів",
      "21 кадр",
      "22 кадри",
      "112 кадрів",
      "300 кадрів",
    ]);
    expect(viewer.directions(3)).toBe("Куди йде рух (3 вектори)");
  });

  it("translates what the server names in English, and keeps what it does not know", () => {
    const { uk } = messages;
    expect(uk.modeName("p2", "Night and dark scenes")).toBe("Ніч і темні сцени");
    expect(uk.modeName("my1", "My own")).toBe("My own");
    expect(uk.sceneTitle("cdnet/baseline/pedestrians")).toBe("Пішоходи");
    expect(uk.sceneTitle("frames/my_clip")).toBe("My clip");
    expect(uk.sceneAbout(sequences[0])).toMatch(/^Дорога з розміткою/);
    expect(uk.sceneAbout({ ...sequences[0], name: "lab" })).toBe("Road with a car");
    expect(uk.sceneSource({ source: "cdnet", category: "badWeather" })).toBe("CDnet · негода");
    expect(uk.sceneSource({ source: "cdnet", category: "PTZ" })).toBe("CDnet · ptz");
    expect(messages.en.sceneAbout({ ...sequences[0], source: "frames" })).toBe("");
    expect(runLabel(summary({ noise: { type: "salt_pepper", level: 0.02, seed: 1 } }), uk)).toBe(
      "Вулиця · Базове віднімання фону · сіль і перець 0.02",
    );
  });

  it("switches the whole app and remembers it", async () => {
    mockFetch({
      "GET /api/pipelines": pipelines,
      "GET /api/sequences": sequences,
      "GET /api/runs": [listItem("run-a")],
    });
    renderIn("en", <App />);
    expect(await screen.findByRole("heading", { name: "Street" })).toBeDefined();
    expect(document.documentElement.lang).toBe("en");

    act(() => {
      fireEvent.click(screen.getByRole("button", { name: "Українська" }));
    });
    expect(screen.getByRole("heading", { name: "Вулиця" })).toBeDefined();
    expect(screen.getByRole("heading", { name: "Сцени" })).toBeDefined();
    expect(screen.getByRole("link", { name: "Історія" })).toBeDefined();
    expect(screen.getByRole("link", { name: "Дивитись: Вулиця" })).toBeDefined();
    expect(screen.getByText("160×120 · 150 кадрів")).toBeDefined();
    expect(screen.getByText("Базове віднімання фону")).toBeDefined();
    expect(document.documentElement.lang).toBe("uk");
    expect(localStorage.getItem("heatmap.lang")).toBe("uk");
    expect(screen.getByRole("button", { name: "Українська" }).getAttribute("aria-pressed")).toBe(
      "true",
    );

    act(() => {
      fireEvent.click(screen.getByRole("button", { name: "English" }));
    });
    expect(screen.getByRole("heading", { name: "Scenes" })).toBeDefined();
  });

  it("shows the viewer in Ukrainian", async () => {
    mockFetch({ "GET /api/runs/r1": { run_id: "r1", status: "done", run: run() } });
    renderIn("uk", <Viewer id="r1" />);
    expect(await screen.findByRole("heading", { name: "Вулиця" })).toBeDefined();
    expect(screen.getByText(/Базове віднімання фону · кадри 0–2/)).toBeDefined();
    expect(screen.getByText("Рух зараз")).toBeDefined();
    expect(screen.getByText("512 кадр/с")).toBeDefined();
    expect(screen.getByText("30.1 дБ")).toBeDefined();
    expect(
      screen.getByText(/F1 на 2 розмічених кадрах · влучність 0.900 · повнота 0.800/),
    ).toBeDefined();
    expect(screen.getByRole("button", { name: "Сліди" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: "пауза" })).toBeDefined();
    expect(screen.getByRole("slider", { name: "кадр" })).toBeDefined();
    expect(screen.getByAltText("маска руху")).toBeDefined();
    fireEvent.click(screen.getByRole("tab", { name: "Графіки" }));
    expect(screen.getByText("Точність за кадрами")).toBeDefined();
    expect(document.body.textContent).not.toMatch(/pipeline|пайплайн/i);
  });

  it("shows the comparison in Ukrainian", async () => {
    mockFetch({ "GET /api/runs": [listItem("a"), listItem("b")] });
    renderIn("uk", <Compare ids={["a", "b"]} />);
    expect(await screen.findByRole("table", { name: "порівняння" })).toBeDefined();
    expect(screen.getByRole("heading", { name: "Порівняння" })).toBeDefined();
    expect(screen.getAllByAltText("Теплова карта: Вулиця · Базове віднімання фону")).toHaveLength(
      2,
    );
  });
});
