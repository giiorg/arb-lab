import { test, expect } from "bun:test";
import { readGuide, loadPure } from "./extract.js";

test("has a title element", () => {
  expect(readGuide()).toMatch(/<title>[^<]+<\/title>/);
});

test("carries no document wrapper tags (the artifact platform supplies them)", () => {
  const html = readGuide();
  expect(html).not.toMatch(/<!DOCTYPE/i);
  expect(html).not.toMatch(/<html[\s>]/i);
  expect(html).not.toMatch(/<head[\s>]/i);
  expect(html).not.toMatch(/<body[\s>]/i);
});

test("references no external resources (strict CSP)", () => {
  const html = readGuide();
  expect(html).not.toMatch(/(?:src|href)\s*=\s*["']https?:/i);
  expect(html).not.toMatch(/(?:src|href)\s*=\s*["']\/\//);
  expect(html).not.toMatch(/@import/i);
});

test("makes no network calls", () => {
  const html = readGuide();
  expect(html).not.toMatch(/\bfetch\s*\(/);
  expect(html).not.toMatch(/XMLHttpRequest/);
  expect(html).not.toMatch(/new\s+WebSocket/);
});

test("states the honest build status", () => {
  expect(readGuide()).toMatch(/spec v0\.9\.4 · 0 lines of Go/);
});

test("defines a pure script block", () => {
  expect(readGuide()).toMatch(/<script id="guide-pure">/);
});

function routesInMarkup(html, attr) {
  return new Set([...html.matchAll(new RegExp(`${attr}="([^"]+)"`, "g"))].map((m) => m[1]));
}

test("every declared route has exactly one content article", () => {
  const pure = loadPure();
  const articles = routesInMarkup(readGuide(), "data-route");
  for (const r of pure.ROUTES) expect(articles.has(r.route)).toBe(true);
});

test("every content article is a declared route", () => {
  const declared = new Set(loadPure().ROUTES.map((r) => r.route));
  for (const route of routesInMarkup(readGuide(), "data-route")) {
    expect(declared.has(route)).toBe(true);
  }
});

test("route ids are unique", () => {
  const routes = loadPure().ROUTES.map((r) => r.route);
  expect(new Set(routes).size).toBe(routes.length);
});

test("every route belongs to one of the six parts", () => {
  const parts = new Set(["world", "edge", "why", "build", "go", "runbook"]);
  for (const r of loadPure().ROUTES) expect(parts.has(r.part)).toBe(true);
});
