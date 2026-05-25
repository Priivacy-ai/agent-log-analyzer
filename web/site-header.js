(function () {
  const links = [
    { id: "home", label: "Home", path: "index.html" },
    { id: "proof", label: "Proof", path: "proof/index.html" },
    { id: "results", label: "Results", path: "proof/results.html" },
    { id: "methodology", label: "Methodology", path: "proof/methodology.html" },
    { id: "benchmark", label: "Benchmark Landscape", path: "proof/benchmark-comparison.html" },
    { id: "security", label: "Security", path: "security/index.html" },
    { id: "privacy", label: "Privacy", path: "privacy/index.html" },
  ];

  function joinPath(root, path) {
    const prefix = root || "";
    if (!prefix || prefix === ".") {
      return path;
    }
    return `${prefix.replace(/\/?$/, "/")}${path}`;
  }

  class SiteHeader extends HTMLElement {
    connectedCallback() {
      const root = this.getAttribute("data-root") || "";
      const active = this.getAttribute("data-active") || "";

      const nav = document.createElement("nav");
      nav.className = "site-header";
      nav.setAttribute("aria-label", "Site navigation");

      for (const link of links) {
        const anchor = document.createElement("a");
        anchor.href = joinPath(root, link.path);
        anchor.textContent = link.label;
        if (link.id === active) {
          anchor.setAttribute("aria-current", "page");
        }
        nav.append(anchor);
      }

      this.replaceChildren(nav);
    }
  }

  customElements.define("site-header", SiteHeader);
})();
