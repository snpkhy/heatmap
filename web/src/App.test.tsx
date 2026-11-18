import { ThemeProvider } from "@mui/material/styles";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { App } from "./App";
import { listItem, mockFetch, pipelines, sequences } from "./test/helpers";
import { theme } from "./theme";

function renderApp() {
  return render(
    <ThemeProvider theme={theme}>
      <App />
    </ThemeProvider>,
  );
}

beforeEach(() => {
  mockFetch({
    "GET /api/pipelines": pipelines,
    "GET /api/sequences": sequences,
    "GET /api/runs": [listItem("run-a")],
  });
});

describe("App", () => {
  it("renders the title and the color scheme toggle", () => {
    renderApp();
    expect(screen.getByRole("heading", { name: "heatmap" })).toBeDefined();
    expect(screen.getByRole("button", { name: "toggle color scheme" })).toBeDefined();
  });

  it("switches between light and dark mode", () => {
    renderApp();
    const toggle = screen.getByRole("button", { name: "toggle color scheme" });

    // Light (or system light in jsdom): the button offers dark mode.
    expect(screen.queryByTestId("DarkModeIcon")).not.toBeNull();

    act(() => {
      fireEvent.click(toggle);
    });
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(screen.queryByTestId("LightModeIcon")).not.toBeNull();

    act(() => {
      fireEvent.click(toggle);
    });
    expect(document.documentElement.classList.contains("light")).toBe(true);
    expect(screen.queryByTestId("DarkModeIcon")).not.toBeNull();
  });

  it("routes between pages with the hash", async () => {
    renderApp();
    expect(await screen.findByRole("heading", { name: "New run" })).toBeDefined();
    act(() => {
      window.location.hash = "#/runs";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(await screen.findByRole("heading", { name: "Runs" })).toBeDefined();
    expect(screen.getByRole("tab", { name: "Runs", selected: true })).toBeDefined();
  });
});
