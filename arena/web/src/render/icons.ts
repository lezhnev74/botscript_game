const NS = "http://www.w3.org/2000/svg";

const SHAPES: Record<string, string[]> = {
  kills: ["circle:8,8,5", "path:M8 0v5M8 11v5M0 8h5M11 8h5"],
  deaths: ["path:M8 1C4 1 2 4 2 7c0 2 1 3 2 4v3h2v-1h1v1h2v-1h1v1h2v-3c1-1 2-2 2-4 0-3-2-6-6-6z", "circle:5.5,7.5,1.5,k", "circle:10.5,7.5,1.5,k"],
};

/** Small inline SVG icon ("kills" crosshair, "deaths" skull); stroke/fill follow currentColor. */
export function svgIcon(kind: "kills" | "deaths"): SVGSVGElement {
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 16 16");
  svg.setAttribute("width", "14");
  svg.setAttribute("height", "14");
  svg.style.verticalAlign = "middle";
  svg.setAttribute("fill", "none");
  svg.setAttribute("stroke", "currentColor");
  svg.setAttribute("stroke-width", "1.5");
  for (const spec of SHAPES[kind]!) {
    const [tag, rest] = spec.split(":") as [string, string];
    const el = document.createElementNS(NS, tag === "circle" ? "circle" : "path");
    if (tag === "path") el.setAttribute("d", rest);
    else {
      const [cx, cy, r, knock] = rest.split(",") as [string, string, string, string?];
      el.setAttribute("cx", cx);
      el.setAttribute("cy", cy);
      el.setAttribute("r", r);
      if (knock) el.setAttribute("fill", "currentColor");
    }
    svg.append(el);
  }
  return svg;
}

/** Header cell content: the icon plus a title and aria-label so it reads as text too. */
export function headerIcon(kind: "kills" | "deaths" | "coins"): HTMLElement {
  const span = document.createElement("span");
  span.title = kind;
  span.setAttribute("aria-label", kind);
  if (kind === "coins") {
    const img = document.createElement("img");
    img.src = "/coin.png";
    img.alt = kind;
    img.style.cssText = "height:14px;vertical-align:middle;image-rendering:pixelated";
    span.append(img);
  } else {
    span.append(svgIcon(kind));
  }
  return span;
}
