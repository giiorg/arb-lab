import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const GUIDE_PATH = join(dirname(fileURLToPath(import.meta.url)), "..", "index.html");

export function readGuide() {
  return readFileSync(GUIDE_PATH, "utf8");
}

// Extracts the DOM-free script block and evaluates it in isolation.
// The block must define `var GuidePure = (function () { ... })();`
export function loadPure() {
  const html = readGuide();
  const match = html.match(/<script id="guide-pure">([\s\S]*?)<\/script>/);
  if (!match) throw new Error('no <script id="guide-pure"> block found in index.html');
  return new Function(`${match[1]}\n;return GuidePure;`)();
}
