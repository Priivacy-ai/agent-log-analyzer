import { resolve } from "node:path";
import { defineConfig } from "vite";

const page = (path) => resolve("web", path);

export default defineConfig({
  root: "web",
  base: "/",
  publicDir: false,
  build: {
    outDir: "../web-dist",
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: {
        home: page("index.html"),
        allowedTools: page("allowed-tools.html"),
        privacy: page("privacy/index.html"),
        security: page("security/index.html"),
        proof: page("proof/index.html"),
        proofMethodology: page("proof/methodology.html"),
        proofResults: page("proof/results.html"),
        proofBenchmarkComparison: page("proof/benchmark-comparison.html"),
        tippy: page("vendor/tippy/tippy.css"),
      },
    },
  },
});
