import { ThemeProvider } from "@mui/material/styles";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { App } from "./App";
import { theme } from "./theme";

function renderApp() {
  return render(
    <ThemeProvider theme={theme}>
      <App />
    </ThemeProvider>,
  );
}

afterEach(() => {
  cleanup();
  localStorage.clear();
  document.documentElement.className = "";
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
});
