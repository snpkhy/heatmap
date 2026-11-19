import { describe, expect, it } from "vitest";
import { previewURL, runFile } from "./api";
import {
  compass,
  decibels,
  fps,
  metric,
  noiseLabel,
  percent,
  sceneSource,
  sceneTitle,
  stageLabel,
} from "./format";
import { noiseString, posterFrame, sceneOrder } from "./pages/Scenes";
import { parseRoute, type Route, routeHref } from "./router";
import { sequences } from "./test/helpers";

describe("format", () => {
  it("formats metrics and handles missing values", () => {
    expect(metric(0.84712)).toBe("0.847");
    expect(metric(null)).toBe("—");
    expect(metric(Number.NaN)).toBe("—");
    expect(fps(512.3)).toBe("512");
    expect(fps(42.25)).toBe("42.3");
    expect(fps(undefined)).toBe("—");
    expect(decibels(30.06)).toBe("30.1 dB");
    expect(decibels(null)).toBe("—");
    expect(percent(0.034)).toBe("3.4%");
    expect(percent(0.48)).toBe("48%");
    expect(percent(null)).toBe("—");
  });

  it("labels noise settings", () => {
    expect(noiseLabel({ type: "none", level: 0, seed: 1 })).toBe("none");
    expect(noiseLabel({ type: "gaussian", level: 10, seed: 1 })).toBe("gaussian 10");
    expect(noiseLabel({ type: "salt_pepper", level: 0.02, seed: 1 })).toBe("salt & pepper 0.02");
    expect(noiseLabel("gaussian:5")).toBe("gaussian:5");
    expect(noiseLabel("")).toBe("none");
    expect(noiseLabel(undefined)).toBe("none");
    expect(noiseString("none", 3)).toBe("none");
    expect(noiseString("gaussian", 12)).toBe("gaussian:12");
  });

  it("names directions and steps", () => {
    expect(compass(0)).toBe("→ right");
    expect(compass(90)).toBe("↑ up");
    expect(compass(-45)).toBe("↘ down-right");
    expect(compass(359)).toBe("→ right");
    expect(stageLabel("2-running_avg_otsu")).toBe("2. running avg otsu");
    expect(stageLabel("mask")).toBe("mask");
  });

  it("titles scenes", () => {
    expect(sceneTitle("synthetic/plaza")).toBe("Plaza");
    expect(sceneTitle("cdnet/shadow/peopleInShade")).toBe("People in shade");
    expect(sceneTitle("tramCrossroad_1fps")).toBe("Tram crossroad 1fps");
    expect(sceneTitle("PETS2006")).toBe("PETS2006");
    expect(sceneTitle("my-clip")).toBe("My clip");
    expect(sceneSource({ source: "synthetic", category: "synthetic" })).toBe("Built-in");
    expect(sceneSource({ source: "frames", category: "custom" })).toBe("Your footage");
    expect(sceneSource({ source: "cdnet", category: "badWeather" })).toBe("CDnet · bad weather");
  });
});

describe("scenes", () => {
  const street = sequences[0];
  const clip = { ...street, id: "frames/clip", source: "frames" as const, has_gt: false };
  const plaza = { ...street, id: "synthetic/plaza", frames: 360 };

  it("puts real footage first, then the longest built-in scenes", () => {
    expect([street, plaza, clip].sort(sceneOrder).map((s) => s.id)).toEqual([
      "frames/clip",
      "synthetic/plaza",
      "synthetic/street",
    ]);
  });

  it("picks a poster frame from the part worth watching", () => {
    expect(posterFrame(street)).toBe(79);
    expect(posterFrame(clip)).toBe(75);
  });
});

describe("router", () => {
  it("parses and builds hash routes", () => {
    expect(parseRoute("")).toEqual({ page: "launch" });
    expect(parseRoute("#/")).toEqual({ page: "launch" });
    expect(parseRoute("#/runs")).toEqual({ page: "runs" });
    expect(parseRoute("#/runs/abc-1")).toEqual({ page: "run", id: "abc-1" });
    expect(parseRoute("#/compare?ids=a,b")).toEqual({ page: "compare", ids: ["a", "b"] });
    expect(parseRoute("#/compare")).toEqual({ page: "compare", ids: [] });
    expect(parseRoute("#/whatever")).toEqual({ page: "launch" });
    const routes: Route[] = [
      { page: "launch" },
      { page: "runs" },
      { page: "run", id: "x-1" },
      { page: "compare", ids: ["a", "b"] },
      { page: "compare", ids: [] },
    ];
    for (const r of routes) {
      expect(parseRoute(routeHref(r))).toEqual(r);
    }
  });
});

describe("api urls", () => {
  it("encodes ids", () => {
    expect(runFile("r 1", "frames/3/mask.png")).toBe("/api/runs/r%201/files/frames/3/mask.png");
    expect(previewURL("synthetic/street", 5)).toBe(
      "/api/sequences/preview?id=synthetic%2Fstreet&frame=5",
    );
  });
});
