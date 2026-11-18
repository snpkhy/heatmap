import { useEffect, useState } from "react";

// A tiny hash router: #/ (new run), #/runs, #/runs/<id>, #/compare.
// Hash routes need no server support and no extra dependency.

export type Route =
  | { page: "launch" }
  | { page: "runs" }
  | { page: "run"; id: string }
  | { page: "compare"; ids: string[] };

export function parseRoute(hash: string): Route {
  const [path, query = ""] = hash.replace(/^#/, "").split("?");
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "runs" && parts[1]) {
    return { page: "run", id: decodeURIComponent(parts[1]) };
  }
  if (parts[0] === "runs") {
    return { page: "runs" };
  }
  if (parts[0] === "compare") {
    const ids = new URLSearchParams(query).get("ids");
    return { page: "compare", ids: ids ? ids.split(",").filter(Boolean) : [] };
  }
  return { page: "launch" };
}

export function routeHref(route: Route): string {
  switch (route.page) {
    case "runs":
      return "#/runs";
    case "run":
      return `#/runs/${encodeURIComponent(route.id)}`;
    case "compare":
      return route.ids.length ? `#/compare?ids=${route.ids.join(",")}` : "#/compare";
    default:
      return "#/";
  }
}

export function navigate(route: Route) {
  window.location.hash = routeHref(route);
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    const onChange = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return route;
}
