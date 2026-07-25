import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

test("reads a language segment out of the hash", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/ka/edge/costs")).toEqual({ lang: "ka", route: "edge/costs" });
});

test("treats a hash with no language segment as English (existing links keep working)", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/edge/costs")).toEqual({ lang: "en", route: "edge/costs" });
});

test("falls back to the first route when the hash is empty", () => {
  const pure = loadPure();
  const first = pure.ROUTES[0].route;
  expect(pure.parseHash("")).toEqual({ lang: "en", route: first });
  expect(pure.parseHash("#/")).toEqual({ lang: "en", route: first });
});

test("a bare language segment still resolves to the first route", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/ka")).toEqual({ lang: "ka", route: pure.ROUTES[0].route });
});

test("only exactly en or ka counts as a language segment", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/world/perps").route).toBe("world/perps");
  expect(pure.parseHash("#/english/perps").route).toBe("english/perps");
});

test("English omits its segment, Georgian carries one", () => {
  const pure = loadPure();
  expect(pure.formatHash({ lang: "en", route: "edge/costs" })).toBe("#/edge/costs");
  expect(pure.formatHash({ lang: "ka", route: "edge/costs" })).toBe("#/ka/edge/costs");
});

test("an unknown language formats as the default", () => {
  const pure = loadPure();
  expect(pure.formatHash({ lang: "de", route: "edge/costs" })).toBe("#/edge/costs");
});

test("parse and format round-trip for every route in both languages", () => {
  const pure = loadPure();
  for (const lang of pure.LANGS) {
    for (const r of pure.ROUTES) {
      const state = { lang: lang, route: r.route };
      expect(pure.parseHash(pure.formatHash(state))).toEqual(state);
    }
  }
});
