const onboardingEl = document.querySelector("#onboarding");
const reportEl = document.querySelector("#report");
const launchPanel = document.querySelector("#launch-panel");
const generateButton = document.querySelector("#generate-session");
const sessionPanel = document.querySelector("#session-panel");
const sessionStatus = document.querySelector("#session-status");
const promptBlock = document.querySelector("#claude-prompt");
const commandBlock = document.querySelector("#curl-command");
const copyPromptButton = document.querySelector("#copy-prompt");
const copyCommandButton = document.querySelector("#copy-command");
const unlockPaidButton = document.querySelector("#unlock-paid");
const waiverAccepted = document.querySelector("#waiver-accepted");
const paidStatus = document.querySelector("#paid-status");
const paidCommand = document.querySelector("#paid-command");
const copyPaidCommandButton = document.querySelector("#copy-paid-command");

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
  generateButton.textContent = "Generating commands...";
  setSessionStatus("", true);
  promptBlock.textContent = analyzeCommand();
  commandBlock.textContent = uploadCommand();
  sessionPanel.hidden = false;
  launchPanel.hidden = true;
  setSessionStatus("No upload token. Step 1 writes a local sanitized report; Step 2 uploads only that report JSON.");
});

copyPromptButton?.addEventListener("click", () => copyText(promptBlock.textContent, copyPromptButton));
copyCommandButton?.addEventListener("click", () => copyText(commandBlock.textContent, copyCommandButton));
copyPaidCommandButton?.addEventListener("click", () => copyText(paidCommand.textContent, copyPaidCommandButton));

unlockPaidButton?.addEventListener("click", async () => {
  unlockPaidButton.disabled = true;
  paidStatus.textContent = "creating waiver-gated paid scan token";
  try {
    const session = await createPaidSession();
    paidCommand.textContent = session.prompt;
    copyPaidCommandButton.hidden = false;
    paidStatus.textContent =
      `paid token expires ${new Date(session.expires_at).toLocaleTimeString()} - review this legacy command before running it`;
    pollPaidJob(session.job_id, session.report_path);
  } catch (error) {
    paidStatus.textContent = `could not unlock paid scan: ${error.message}`;
    unlockPaidButton.disabled = false;
  }
});

async function createSession() {
  const response = await fetch("/api/analysis-sessions", { method: "POST" });
  if (!response.ok) {
    throw await responseError(response);
  }
  return response.json();
}

function analyzeCommand() {
  return [
    "go install \\",
    "  github.com/robertdouglass/claude-log-analyzer/cmd/claude-analyzer@v0.1.0",
    "# omit the path to use the latest log under ~/.claude/projects/,",
    "# or pass a path positionally (--log <path> also works):",
    "claude-analyzer analyze --out ./claude-analyzer-report.json",
  ].join("\n");
}

function uploadCommand() {
  const baseURL = window.location.origin && window.location.origin !== "null"
    ? window.location.origin
    : "https://claude-code.spec-kitty.ai";
  return [
    "jq . ./claude-analyzer-report.json",
    "claude-analyzer upload \\",
    `  --base-url ${baseURL} \\`,
    "  ./claude-analyzer-report.json",
  ].join("\n");
}

async function createPaidSession() {
  const acknowledgment =
    "I understand that Claude Analyzer provides deterministic analysis and vetted setup recommendations, but any installation or code change is executed by Claude Code, my package manager, or third-party tools with my approval and at my own risk.";
  const response = await fetch("/api/paid-sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      waiver_accepted: Boolean(waiverAccepted?.checked),
      acknowledgment,
    }),
  });
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
      setSessionStatus("Ready. Paste Step 1 into Claude Code; this page will update after Claude uploads the session.");
    } else if (job.status === "pending" || job.status === "processing") {
      setSessionStatus("Analyzing uploaded session.");
    } else if (job.status === "completed") {
      setSessionStatus(`Report ready: <a href="${reportPath}">${reportPath}</a>`, false, true);
      return;
    } else if (job.status === "failed") {
      setSessionStatus("Analysis failed.");
      return;
    }
    await sleep(1000);
  }
}

async function pollPaidJob(jobID, reportPath) {
  for (;;) {
    const response = await fetch(`/api/jobs/${jobID}`);
    const job = await response.json();
    if (job.status === "uploading") {
      paidStatus.textContent = "waiting for paid scan upload";
    } else if (job.status === "pending" || job.status === "processing") {
      paidStatus.textContent = "analyzing paid scan bundle";
    } else if (job.status === "completed") {
      paidStatus.innerHTML = `paid report ready: <a href="${reportPath}">${reportPath}</a>`;
      return;
    } else if (job.status === "failed") {
      paidStatus.textContent = "paid analysis failed";
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
    findings.appendChild(buildFindingItem(finding));
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
  renderWorkflowFingerprints(report);
  renderToolingUtilization(report);
  document.querySelector("#ecosystem").textContent = summarizeEcosystem(report.ecosystem);
  document.querySelector("#receipt").textContent = summarizeReceipt(report.security_receipt);
  renderPaidCommandPreview(report);
}

function buildFindingItem(finding) {
  const item = document.createElement("li");

  const title = document.createElement("strong");
  title.textContent = typeof finding?.title === "string" ? finding.title : "";
  item.appendChild(title);

  const meta = document.createElement("span");
  const severity = typeof finding?.severity === "string" ? finding.severity : "unknown";
  const impact = typeof finding?.cost_impact === "string" ? finding.cost_impact : "unknown";
  meta.textContent = `${severity} - ${impact}`;
  item.appendChild(meta);

  const evidence = document.createElement("p");
  evidence.textContent = findingEvidence(finding?.evidence);
  item.appendChild(evidence);

  const recommendation = document.createElement("p");
  recommendation.textContent = typeof finding?.recommendation === "string" ? finding.recommendation : "";
  item.appendChild(recommendation);

  return item;
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

function renderWorkflowFingerprints(report) {
  const section = document.querySelector("#workflow-fingerprints");
  const list = document.querySelector("#workflow-fingerprints-list");
  if (!section || !list) return;
  const fps = report && report.ecosystem && Array.isArray(report.ecosystem.workflow_fingerprints)
    ? report.ecosystem.workflow_fingerprints
    : [];
  list.replaceChildren();
  if (fps.length === 0) {
    section.hidden = true;
    return;
  }
  section.hidden = false;
  for (const fp of fps) {
    if (!fp || typeof fp !== "object") continue;
    const row = document.createElement("li");
    row.className = "fingerprint-row";

    const title = document.createElement("span");
    title.className = "fingerprint-id";
    title.textContent = typeof fp.id === "string" ? fp.id : "";
    row.appendChild(title);

    const confidence = document.createElement("span");
    const confValue = typeof fp.confidence === "string" ? fp.confidence : "";
    confidence.className = "fingerprint-confidence";
    if (confValue) confidence.classList.add(`confidence-${confValue}`);
    confidence.textContent = confValue;
    row.appendChild(confidence);

    if (Array.isArray(fp.sources) && fp.sources.length > 0) {
      const sources = document.createElement("ul");
      sources.className = "fingerprint-sources";
      for (const source of fp.sources) {
        const item = document.createElement("li");
        item.textContent = typeof source === "string" ? source : "";
        sources.appendChild(item);
      }
      row.appendChild(sources);
    }

    const evidence = document.createElement("span");
    evidence.className = "fingerprint-evidence";
    const evCount = typeof fp.evidence_count === "number" ? fp.evidence_count : 0;
    evidence.textContent = `evidence: ${evCount}`;
    row.appendChild(evidence);

    if (fp.active === true) {
      const active = document.createElement("span");
      active.className = "fingerprint-active";
      active.textContent = "active";
      row.appendChild(active);
    }
    if (fp.installed === true) {
      const installed = document.createElement("span");
      installed.className = "fingerprint-installed";
      installed.textContent = "installed";
      row.appendChild(installed);
    }
    if (typeof fp.version_bucket === "string" && fp.version_bucket.length > 0) {
      const version = document.createElement("span");
      version.className = "fingerprint-version";
      version.textContent = `version: ${fp.version_bucket}`;
      row.appendChild(version);
    }

    list.appendChild(row);
  }
}

// The four allowlisted advice IDs are emitted by internal/analyzer/analyzer.go:368-394.
// If those IDs change, this UI must be updated in lockstep.
const ADVICE_LOOKUP = {
  mcp: { severe: "mcp_bloat_severe", high: "mcp_bloat_high" },
  skill: { severe: "skill_bloat_severe", high: "skill_bloat_high" },
};
// No keys for watch/normal/unknown — structurally enforces FR-006.

function findingById(report, id) {
  const list = report && Array.isArray(report.findings) ? report.findings : [];
  return list.find((f) => f && f.id === id) || null;
}

// Empty/missing warning_band → render as "unknown" (matches analyzer guarantee
// in tooling_classify.go:149-151 / 191-193 when exposure_known is false; also
// tolerates a future struct-default zero-value).
function normalizeBand(b) {
  const v = typeof b === "string" ? b : "";
  return (v === "severe" || v === "high" || v === "watch" || v === "normal") ? v : "unknown";
}

function renderToolingUtilization(report) {
  const section = document.querySelector("#tooling-utilization");
  const rowsRoot = document.querySelector("#tooling-utilization-rows");
  if (!section || !rowsRoot) return;
  const tu = report && report.ecosystem && report.ecosystem.tooling_utilization;
  rowsRoot.replaceChildren();
  if (!tu) {
    section.hidden = true;
    return;
  }
  section.hidden = false;

  const mcp = tu.mcp;
  if (mcp && typeof mcp === "object") {
    rowsRoot.appendChild(buildMCPRow(report, mcp));
  }
  const skill = tu.skill;
  if (skill && typeof skill === "object") {
    rowsRoot.appendChild(buildSkillRow(report, skill));
  }
}

function buildMCPRow(report, mcp) {
  const row = document.createElement("div");
  row.className = "utilization-row";

  const header = document.createElement("div");
  header.className = "surface-header";
  header.textContent = "MCP";
  row.appendChild(header);

  const body = document.createElement("div");
  body.className = "surface-body";

  // Bucket cells.
  appendBucket(body, "servers", mcp.server_count_bucket);
  appendBucket(body, "exposed tools", mcp.exposed_tool_count_bucket);
  appendBucket(body, "context tokens", mcp.context_token_bucket);
  appendBucket(body, "context efficiency", mcp.context_efficiency_bucket);

  // Counts (numeric only — never names).
  appendCount(body, "calls", mcp.call_count);
  appendCount(body, "known calls", mcp.known_call_count);
  appendCount(body, "unknown calls", mcp.unknown_call_count);
  appendCount(body, "unknown servers", mcp.unknown_server_count);
  appendCount(body, "unique unknown called", mcp.unique_unknown_called_count);
  appendCount(
    body,
    "unique known called",
    Array.isArray(mcp.unique_known_called_ids) ? mcp.unique_known_called_ids.length : 0,
  );
  appendCount(
    body,
    "known servers",
    Array.isArray(mcp.known_server_ids) ? mcp.known_server_ids.length : 0,
  );

  // Band chip.
  const band = normalizeBand(mcp.warning_band);
  const chip = document.createElement("span");
  chip.className = `band-chip band-${band}`;
  chip.textContent = band;
  body.appendChild(chip);

  // Ratio cell (FR-007).
  const ratio = document.createElement("span");
  ratio.className = "utilization-ratio";
  if (mcp.exposure_known === true) {
    const pct = typeof mcp.utilization_ratio_pct === "number" ? mcp.utilization_ratio_pct : 0;
    ratio.textContent = `${pct}%`;
  } else {
    const src = typeof mcp.inference_source === "string" ? mcp.inference_source : "";
    ratio.textContent = `inferred from: ${src}`;
  }
  body.appendChild(ratio);

  row.appendChild(body);

  // Advice block (FR-005 / FR-006).
  const adviceId = ADVICE_LOOKUP.mcp[band];
  const finding = adviceId ? findingById(report, adviceId) : null;
  if (finding && typeof finding.recommendation === "string" && finding.recommendation.length > 0) {
    const advice = document.createElement("p");
    advice.className = "band-advice";
    advice.textContent = finding.recommendation;
    row.appendChild(advice);
  }

  return row;
}

function buildSkillRow(report, skill) {
  const row = document.createElement("div");
  row.className = "utilization-row";

  const header = document.createElement("div");
  header.className = "surface-header";
  header.textContent = "Skill";
  row.appendChild(header);

  const body = document.createElement("div");
  body.className = "surface-body";

  // Bucket cells (Skill has no exposed_tool_count_bucket).
  appendBucket(body, "exposed", skill.exposed_count_bucket);
  appendBucket(body, "context tokens", skill.context_token_bucket);
  appendBucket(body, "context efficiency", skill.context_efficiency_bucket);

  // Counts.
  appendCount(body, "executed", skill.executed_count);
  appendCount(body, "unknown exposed", skill.unknown_exposed_count);
  appendCount(body, "unknown executed", skill.unknown_executed_count);
  appendCount(
    body,
    "known exposed",
    Array.isArray(skill.known_exposed_ids) ? skill.known_exposed_ids.length : 0,
  );
  appendCount(
    body,
    "known executed",
    Array.isArray(skill.known_executed_ids) ? skill.known_executed_ids.length : 0,
  );

  // Band chip.
  const band = normalizeBand(skill.warning_band);
  const chip = document.createElement("span");
  chip.className = `band-chip band-${band}`;
  chip.textContent = band;
  body.appendChild(chip);

  // Ratio cell (FR-007).
  const ratio = document.createElement("span");
  ratio.className = "utilization-ratio";
  if (skill.exposure_known === true) {
    const pct = typeof skill.utilization_ratio_pct === "number" ? skill.utilization_ratio_pct : 0;
    ratio.textContent = `${pct}%`;
  } else {
    const src = typeof skill.inference_source === "string" ? skill.inference_source : "";
    ratio.textContent = `inferred from: ${src}`;
  }
  body.appendChild(ratio);

  row.appendChild(body);

  // Advice block.
  const adviceId = ADVICE_LOOKUP.skill[band];
  const finding = adviceId ? findingById(report, adviceId) : null;
  if (finding && typeof finding.recommendation === "string" && finding.recommendation.length > 0) {
    const advice = document.createElement("p");
    advice.className = "band-advice";
    advice.textContent = finding.recommendation;
    row.appendChild(advice);
  }

  return row;
}

function appendBucket(parent, label, value) {
  const span = document.createElement("span");
  span.className = "bucket-cell";
  const v = typeof value === "string" && value.length > 0 ? value : "—";
  span.textContent = `${label}: ${v}`;
  parent.appendChild(span);
}

function appendCount(parent, label, value) {
  const span = document.createElement("span");
  span.className = "count-cell";
  const n = typeof value === "number" ? value : 0;
  span.textContent = `${label}: ${n}`;
  parent.appendChild(span);
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

function renderPaidCommandPreview(report) {
  const target = document.querySelector("#paid-command");
  if (!target) return;
  if (report?.aggregate_event?.parser_type === "paid_bundle" && route) {
    const pluginURL = `${window.location.origin}/api/public-artifacts/${route.jobID}/${route.token}/plugin.zip`;
    const command = [
      `PLUGIN_URL="${pluginURL}"`,
      'PLUGIN_ZIP="$(mktemp -t claude-analyzer-plugin.XXXXXX.zip)"',
      'curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"',
      'claude --plugin-dir "$PLUGIN_ZIP"',
    ].join("\n");
    target.textContent =
      "Install the generated Claude Analyzer optimization plugin for this session. Run the command below, explain what it installs, summarize the waiver, and ask for approval before executing it. Do not print plugin archive contents.\n\n```sh\n" +
      command +
      "\n```";
    if (paidStatus) paidStatus.textContent = "optimization plugin ready; artifact URL expires with this report";
    if (copyPaidCommandButton) copyPaidCommandButton.hidden = false;
    if (unlockPaidButton) unlockPaidButton.hidden = true;
    if (waiverAccepted) waiverAccepted.closest("label").hidden = true;
    const upsellCopy = document.querySelectorAll(".upsell p");
    if (upsellCopy[0]) {
      upsellCopy[0].textContent =
        "Your paid bundle scan is complete. The optimization plugin below is generated from sanitized aggregate findings and vetted tooling recommendations.";
    }
    if (upsellCopy[1]) {
      upsellCopy[1].textContent =
        "Review the install command with Claude Code. Claude should summarize the waiver and ask before running it.";
    }
    return;
  }
  target.textContent = "Accept the waiver and unlock to generate the paid local-first scan commands.";
}

function parseReportRoute() {
  const match = window.location.pathname.match(/^\/r\/([^/]+)\/([^/]+)$/);
  if (!match) return null;
  return { jobID: match[1], token: match[2] };
}

function setReportStatus(message) {
  document.querySelector("#report-status").textContent = message;
}

function setSessionStatus(message, hidden = false, html = false) {
  if (!sessionStatus) return;
  sessionStatus.hidden = hidden;
  if (html) {
    sessionStatus.innerHTML = message;
  } else {
    sessionStatus.textContent = message;
  }
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
