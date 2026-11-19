import { ThemeProvider } from "@mui/material/styles";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { App } from "./App";
import { listItem, mockFetch, pipelines, sequences } from "./test/helpers";
import { theme } from "./theme";

function renderApp() {
  return render(
    <ThemeProvider theme={theme} defaultMode="dark">
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
    expect(screen.getByRole("heading", { name: "Heatmap" })).toBeDefined();
    expect(screen.getByRole("button", { name: "toggle color scheme" })).toBeDefined();
  });

  it("starts dark and switches between light and dark mode", () => {
    renderApp();
    const toggle = screen.getByRole("button", { name: "toggle color scheme" });

    // Dark by default: the button offers light mode.
    expect(screen.queryByTestId("LightModeOutlinedIcon")).not.toBeNull();

    act(() => {
      fireEvent.click(toggle);
    });
    expect(document.documentElement.classList.contains("light")).toBe(true);
    expect(screen.queryByTestId("DarkModeOutlinedIcon")).not.toBeNull();

    act(() => {
      fireEvent.click(toggle);
    });
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(screen.queryByTestId("LightModeOutlinedIcon")).not.toBeNull();
  });

  it("routes between pages with the hash", async () => {
    renderApp();
    expect(await screen.findByRole("heading", { name: "Scenes" })).toBeDefined();
    expect(screen.getByRole("link", { name: "Scenes" }).getAttribute("aria-current")).toBe("page");
    act(() => {
      window.location.hash = "#/runs";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(await screen.findByRole("heading", { name: "History" })).toBeDefined();
    expect(screen.getByRole("link", { name: "History" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: "Scenes" }).getAttribute("aria-current")).toBeNull();
  });
});
