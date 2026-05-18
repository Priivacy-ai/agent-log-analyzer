const onboardingEl = document.querySelector("#onboarding");
const reportEl = document.querySelector("#report");
const generateButton = document.querySelector("#generate-session");
const sessionPanel = document.querySelector("#session-panel");
const sessionStatus = document.querySelector("#session-status");
const promptBlock = document.querySelector("#claude-prompt");
const commandBlock = document.querySelector("#curl-command");
const copyPromptButton = document.querySelector("#copy-prompt");
const copyCommandButton = document.querySelector("#copy-command");

const route = parseReportRoute();

if (route) {
  onboardingEl.hidden = true;
  reportEl.hidden = false;
  pollReport(route.jobID, route.token);
} else {
  reportEl.hidden = true;
}

generateButton?.addEventListener("click", async () => {
  generateButton.disabled = true;
  sessionStatus.textContent = "minting one-time upload token";
  try {
    const session = await createSession();
    promptBlock.textContent = session.prompt;
    commandBlock.textContent = session.command;
    sessionPanel.hidden = false;
    sessionStatus.textContent =
      `token expires ${new Date(session.expires_at).toLocaleTimeString()} - paste the prompt into Claude Code`;
    pollJob(session.job_id, session.report_path);
  } catch (error) {
    sessionStatus.textContent = `could not create session: ${error.message}`;
    generateButton.disabled = false;
  }
});

copyPromptButton?.addEventListener("click", () => copyText(promptBlock.textContent, copyPromptButton));
copyCommandButton?.addEventListener("click", () => copyText(commandBlock.textContent, copyCommandButton));

async function createSession() {
  const response = await fetch("/api/analysis-sessions", { method: "POST" });
  if (!response.ok) {
    throw await responseError(response);
  }
  return response.json();
}

async function pollJob(jobID, reportPath) {
  for (;;) {
    const response = await fetch(`/api/jobs/${jobID}`);
    const job = await response.json();
    if (job.status === "uploading") {
      sessionStatus.textContent = "waiting for Claude Code upload";
    } else if (job.status === "pending" || job.status === "processing") {
      sessionStatus.textContent = "analyzing uploaded session";
    } else if (job.status === "completed") {
      sessionStatus.innerHTML = `report ready: <a href="${reportPath}">${reportPath}</a>`;
      return;
    } else if (job.status === "failed") {
      sessionStatus.textContent = "analysis failed";
      return;
    }
    await sleep(1000);
  }
}

async function pollReport(jobID, token) {
  setReportStatus("This report is visible for 15 minutes. Waiting for analysis.");
  for (;;) {
    const jobResponse = await fetch(`/api/jobs/${jobID}`);
    if (jobResponse.ok) {
      const job = await jobResponse.json();
      if (job.status === "failed") {
        setReportStatus("Analysis failed.");
        return;
      }
      if (job.status !== "completed") {
        setReportStatus(`This report is visible for 15 minutes. Status: ${job.status}.`);
        await sleep(1000);
        continue;
      }
    }
    const reportResponse = await fetch(`/api/public-reports/${jobID}/${token}`);
    if (reportResponse.status === 404) {
      await sleep(1000);
      continue;
    }
    if (!reportResponse.ok) {
      setReportStatus(`Report unavailable: ${(await responseError(reportResponse)).message}`);
      return;
    }
    renderReport(await reportResponse.json());
    renderPaidCommandPreview();
    return;
  }
}

function renderReport(report) {
  document.querySelector("#report-status").textContent = "This report is visible for 15 minutes.";
  document.querySelector("#score").textContent = report.score;
  document.querySelector("#waste").textContent =
    `${report.estimated_waste_pct.low}-${report.estimated_waste_pct.high}% avoidable token spend`;

  const findings = document.querySelector("#findings");
  findings.innerHTML = "";
  for (const finding of report.findings) {
    const item = document.createElement("li");
    item.innerHTML = `<strong>${finding.title}</strong><span>${finding.severity} - ${finding.cost_impact}</span><p>${findingEvidence(finding.evidence)}</p><p>${finding.recommendation}</p>`;
    findings.appendChild(item);
  }
  if (report.findings.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No major deterministic waste pattern detected.";
    findings.appendChild(item);
  }

  const fixes = document.querySelector("#fixes");
  fixes.innerHTML = "";
  for (const fix of report.immediate_fixes || []) {
    const item = document.createElement("li");
    item.textContent = fix;
    fixes.appendChild(item);
  }

  renderTimeline(report.timeline || []);
  document.querySelector("#ecosystem").textContent = summarizeEcosystem(report.ecosystem);
  document.querySelector("#receipt").textContent = summarizeReceipt(report.security_receipt);
}

function renderTimeline(points) {
  const chart = document.querySelector("#timeline");
  chart.innerHTML = "";
  if (points.length === 0) {
    chart.textContent = "No timeline points detected.";
    return;
  }
  const maxTokens = Math.max(...points.map((point) => point.estimated_tokens), 1);
  for (const point of points.slice(-60)) {
    const bar = document.createElement("span");
    bar.style.height = `${Math.max(4, (point.estimated_tokens / maxTokens) * 100)}%`;
    bar.title = `turn ${point.turn}: ${point.estimated_tokens} estimated tokens`;
    chart.appendChild(bar);
  }
}

function summarizeEcosystem(ecosystem) {
  if (!ecosystem) return "No ecosystem signals detected.";
  const parts = [
    `Client: ${ecosystem.client || "unknown"}`,
    `OS: ${ecosystem.operating_system || "unknown"}`,
    `Frameworks: ${(ecosystem.workflow_frameworks || []).join(", ") || "none detected"}`,
    `MCPs: ${(ecosystem.mcp_servers_known || []).join(", ") || "none detected"}`,
  ];
  return parts.join("\n");
}

function summarizeReceipt(receipt) {
  if (!receipt) return "No security receipt available.";
  return [
    `Raw transcript sent to LLM: ${receipt.raw_transcript_sent_to_llm}`,
    `Outbound during analysis: ${receipt.outbound_during_analysis}`,
    `Secrets redacted: ${receipt.secrets_redacted}`,
    `Raw log TTL: ${receipt.raw_log_ttl}`,
  ].join("\n");
}

function findingEvidence(evidence) {
  if (!evidence) return "Deterministic evidence recorded.";
  const parts = [];
  if (evidence.description) parts.push(evidence.description);
  if (evidence.count) parts.push(`count: ${evidence.count}`);
  if (evidence.token_share_pct) parts.push(`token share: ${evidence.token_share_pct}%`);
  if (evidence.top_files && evidence.top_files.length) parts.push(`top files: ${evidence.top_files.join(", ")}`);
  return parts.join(" | ") || "Deterministic evidence recorded.";
}

function renderPaidCommandPreview() {
  const target = document.querySelector("#paid-command");
  if (!target) return;
  target.textContent = [
    "# Generated after Stripe unlock with a separate paid token.",
    "CLAUDE_ANALYZER_SCAN_LIMIT=100",
    "PAID_TOKEN=\"<paid-upload-token>\"",
    "PAID_SESSION=\"<paid-session-id>\"",
    "BUNDLE=\"$(mktemp -t claude-analyzer-logs.XXXXXX.tar.gz)\"",
    "LIST=\"$(mktemp -t claude-analyzer-logs.XXXXXX.txt)\"",
    "python3 - <<'PY' > \"$LIST\"",
    "from pathlib import Path",
    "home = Path.home()",
    "logs = sorted(home.glob('.claude/projects/**/*.jsonl'), key=lambda p: p.stat().st_mtime, reverse=True)[:100]",
    "if not logs:",
    "    raise SystemExit('No Claude Code JSONL logs found under ~/.claude/projects')",
    "for path in logs:",
    "    print(path.relative_to(home))",
    "PY",
    "tar -C \"$HOME\" -czf \"$BUNDLE\" -T \"$LIST\"",
    `curl -fsS -X PUT '${window.location.origin}/api/paid-uploads/$PAID_SESSION?limit=100' \\`,
    "  -H \"Authorization: Bearer $PAID_TOKEN\" \\",
    "  -H 'Content-Type: application/gzip' \\",
    "  -H 'X-Scan-Limit: 100' \\",
    "  --data-binary \"@$BUNDLE\"",
  ].join("\n");
}

function parseReportRoute() {
  const match = window.location.pathname.match(/^\/r\/([^/]+)\/([^/]+)$/);
  if (!match) return null;
  return { jobID: match[1], token: match[2] };
}

function setReportStatus(message) {
  document.querySelector("#report-status").textContent = message;
}

async function responseError(response) {
  const error = new Error(await response.text());
  error.status = response.status;
  return error;
}

async function copyText(text, button) {
  await navigator.clipboard.writeText(text);
  const previous = button.textContent;
  button.textContent = "Copied";
  setTimeout(() => {
    button.textContent = previous;
  }, 1200);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
