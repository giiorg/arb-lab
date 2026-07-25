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
