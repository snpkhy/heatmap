import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// jsdom lacks ResizeObserver, which the charts use to size themselves.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", ResizeObserverStub);

// jsdom has no canvas: the stage and the frame store then simply draw nothing.
HTMLCanvasElement.prototype.getContext = (() =>
  null) as typeof HTMLCanvasElement.prototype.getContext;

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.stubGlobal("ResizeObserver", ResizeObserverStub);
  localStorage.clear();
  document.documentElement.className = "";
  window.location.hash = "";
});
