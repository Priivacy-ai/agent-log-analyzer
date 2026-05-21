(function () {
  async function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "true");
    textarea.style.position = "fixed";
    textarea.style.left = "-9999px";
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    textarea.remove();
  }

  document.addEventListener("click", async function (event) {
    const button = event.target.closest(".copy-agents-line[data-copy]");
    if (!button) return;
    const original = button.textContent;
    try {
      await copyText(button.dataset.copy || "");
      button.textContent = "Copied";
      setTimeout(function () {
        button.textContent = original;
      }, 1200);
    } catch (error) {
      button.textContent = "Copy failed";
      setTimeout(function () {
        button.textContent = original;
      }, 1600);
    }
  });
})();
