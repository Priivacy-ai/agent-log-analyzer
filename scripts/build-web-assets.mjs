import { cp, mkdir } from "node:fs/promises";
import { join } from "node:path";

const passthroughAssets = [
  "app.js",
  "report-actions.js",
  "site-header.js",
  "styles.css",
  "tooltips.js",
  "vendor",
];

await mkdir("web-dist", { recursive: true });

for (const asset of passthroughAssets) {
  await cp(join("web", asset), join("web-dist", asset), {
    recursive: true,
  });
}

await mkdir(join("web-dist", "proof"), { recursive: true });
await cp(join("web", "proof", "results.json"), join("web-dist", "proof", "results.json"));
