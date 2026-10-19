import { ThemeProvider } from "@mui/material/styles";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { theme } from "./theme";

describe("App", () => {
  it("renders the title and the color scheme toggle", () => {
    render(
      <ThemeProvider theme={theme}>
        <App />
      </ThemeProvider>,
    );
    expect(screen.getByRole("heading", { name: "heatmap" })).toBeDefined();
    expect(screen.getByRole("button", { name: "toggle color scheme" })).toBeDefined();
  });
});
