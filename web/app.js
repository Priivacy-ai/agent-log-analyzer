const onboardingEl = document.querySelector("#onboarding");
const reportEl = document.querySelector("#report");
const sessionPanel = document.querySelector("#session-panel");
const sessionStatus = document.querySelector("#session-status");
const promptBlock = document.querySelector("#claude-prompt");
const copyPromptButton = document.querySelector("#copy-prompt");
const unlockPaidButton = document.querySelector("#unlock-paid");
const waiverAccepted = document.querySelector("#waiver-accepted");
const paidStatus = document.querySelector("#paid-status");
const paidCommand = document.querySelector("#paid-command");
const copyPaidCommandButton = document.querySelector("#copy-paid-command");
const emailUnlockJobID = document.querySelector("#email-unlock-job-id");
const emailUnlockReportToken = document.querySelector("#email-unlock-report-token");

const route = parseReportRoute();

if (route) {
  onboardingEl.hidden = true;
  reportEl.hidden = false;
  if (emailUnlockJobID) emailUnlockJobID.value = route.jobID;
  if (emailUnlockReportToken) emailUnlockReportToken.value = route.token;
  pollReport(route.jobID, route.token);
} else {
  reportEl.hidden = true;
  if (promptBlock) promptBlock.textContent = runCommand();
  if (sessionPanel) sessionPanel.hidden = false;
}

copyPromptButton?.addEventListener("click", () => copyText(promptBlock.textContent, copyPromptButton));
copyPaidCommandButton?.addEventListener("click", () => copyText(paidCommand.textContent, copyPaidCommandButton));

unlockPaidButton?.addEventListener("click", async () => {
  unlockPaidButton.disabled = true;
  paidStatus.textContent = "creating waiver-gated paid scan commands";
  try {
    const session = await createPaidSession();
    paidCommand.textContent = session.prompt;
    copyPaidCommandButton.hidden = false;
    if (session.job_id && session.report_path) {
      paidStatus.textContent =
        `paid token expires ${new Date(session.expires_at).toLocaleTimeString()} - review these commands before running them`;
      pollPaidJob(session.job_id, session.report_path);
    } else {
      paidStatus.textContent = "review these local-first commands; they upload only the sanitized aggregate JSON";
    }
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

function runCommand() {
  return [
    "npx --yes agent-analyzer@latest run",
  ].join("\n");
}

async function createPaidSession() {
  const acknowledgment =
    "I understand that Agent Analyzer provides deterministic analysis and vetted setup recommendations, but any installation or code change is executed by Claude Code, my package manager, or third-party tools with my approval and at my own risk.";
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
      paidStatus.textContent = "analyzing sanitized paid scan report";
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
  renderTokenVolume(report);

  const findings = document.querySelector("#findings");
  findings.innerHTML = "";
  findings.className = "problem-bubbles";
  const estimates = (report.findings || []).map((finding) => representativeProblemTokens(finding, report));
  const maxEstimate = Math.max(...estimates, 1);
  for (const [index, finding] of (report.findings || []).entries()) {
    findings.appendChild(buildFindingItem(finding, report, index, estimates[index], maxEstimate));
  }
  if ((report.findings || []).length === 0) {
    findings.classList.add("problem-bubbles-empty");
    const item = document.createElement("p");
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
  if ((report.immediate_fixes || []).length === 0) {
    const item = document.createElement("li");
    item.textContent = "No immediate deterministic fix was required, but the paid scan can still generate broader workflow and tooling guidance from more sessions.";
    fixes.appendChild(item);
  }

  const timelineSection = document.querySelector("#timeline-section");
  if ((report.source_reports || []).length > 0) {
    if (timelineSection) timelineSection.hidden = true;
  } else {
    if (timelineSection) timelineSection.hidden = false;
    renderTimeline(report.timeline || [], report.estimated_waste_pct);
  }
  renderRecommendation(report);
  renderWorkflowFingerprints(report);
  renderToolingUtilization(report);
  renderEcosystem(report.ecosystem);
  renderReceipt(report.security_receipt, report.redactions);
  renderPaidCommandPreview(report);
}

function buildFindingItem(finding, report, index, estimatedTokens, maxEstimate) {
  const item = document.createElement("article");
  item.className = `problem-bubble problem-bubble-${bubbleTone(finding, index)}`;
  const diameter = bubbleDiameter(estimatedTokens, maxEstimate);
  item.style.setProperty("--bubble-size", `${diameter}px`);
  item.style.setProperty("--bubble-offset", `${bubbleOffset(index)}px`);
  item.setAttribute("role", "listitem");
  item.setAttribute(
    "aria-label",
    [
      typeof finding?.title === "string" ? finding.title : "Problem",
      typeof finding?.severity === "string" ? finding.severity : "unknown severity",
      `${formatCompactNumber(estimatedTokens)} representative tokens`,
      findingEvidence(finding?.evidence),
      typeof finding?.recommendation === "string" ? finding.recommendation : "",
    ].filter(Boolean).join(". "),
  );

  const rank = document.createElement("span");
  rank.className = "problem-rank";
  rank.textContent = String(index + 1);
  item.appendChild(rank);

  const title = document.createElement("strong");
  const titleText = typeof finding?.title === "string" ? finding.title : "";
  title.textContent = titleText;
  item.style.setProperty("--problem-title-size", `${bubbleLabelFontSize(titleText, diameter, 21, 10.5)}px`);
  item.appendChild(title);

  const meta = document.createElement("span");
  meta.className = "problem-meta";
  const severity = typeof finding?.severity === "string" ? finding.severity : "unknown";
  const impact = typeof finding?.cost_impact === "string" ? finding.cost_impact : "unknown";
  const metaText = `${severity} - ${impact}`;
  meta.textContent = metaText;
  item.appendChild(meta);

  const estimate = document.createElement("span");
  estimate.className = "problem-impact";
  const estimateText = `${formatCompactNumber(estimatedTokens)} representative tokens`;
  estimate.textContent = estimateText;
  item.style.setProperty("--problem-detail-size", `${bubbleLabelFontSize(`${metaText} ${estimateText}`, diameter, 12, 9.5)}px`);
  item.appendChild(estimate);

  const evidence = document.createElement("p");
  evidence.textContent = findingEvidence(finding?.evidence);
  item.appendChild(evidence);

  const recommendation = document.createElement("p");
  recommendation.textContent = typeof finding?.recommendation === "string" ? finding.recommendation : "";
  item.appendChild(recommendation);

  return item;
}

function representativeProblemTokens(finding, report) {
  const metrics = report?.metrics || {};
  const signals = report?.analysis_signals || {};
  let total = numberValue(metrics.estimated_tokens);
  if (total <= 0) total = numberValue(signals.input_tokens) + numberValue(signals.output_tokens);
  if (total <= 0) total = 1000;
  const evidence = finding?.evidence || {};
  const tokenShare = numberValue(evidence.token_share_pct);
  if (tokenShare > 0) return clampProblemTokens(Math.round(total * tokenShare / 100), total);
  const count = numberValue(evidence.count);
  switch (finding?.id) {
    case "tool_output_bloat":
      return clampProblemTokens(numberValue(metrics.tool_output_tokens) || Math.round(total * 0.25), total);
    case "cache_invalidation_spike":
      return clampProblemTokens(numberValue(signals.cache_creation_tokens) || Math.round(total * 0.22), total);
    case "args_hashed_retry_loop":
      return percentageProblemTokens(total, count, 5, 34);
    case "retry_loop":
      return percentageProblemTokens(total, count || numberValue(metrics.retry_depth_max), 5, 32);
    case "repeated_file_reads":
      return percentageProblemTokens(total, count || numberValue(metrics.rereads), 3, 38);
    case "context_growth_spikes":
      return percentageProblemTokens(total, count || numberValue(metrics.context_growth_events), 4, 42);
    default: {
      const wasteRange = normalizeWasteRange(report?.estimated_waste_pct);
      const wasteMid = Math.max(12, Math.round((wasteRange.low + wasteRange.high) / 2));
      return clampProblemTokens(Math.round(total * wasteMid / 100), total);
    }
  }
}

function percentageProblemTokens(total, count, perCountPct, maxPct) {
  const boundedCount = Math.max(1, numberValue(count));
  const pct = Math.min(maxPct, Math.max(4, boundedCount * perCountPct));
  return clampProblemTokens(Math.round(total * pct / 100), total);
}

function clampProblemTokens(tokens, total) {
  return Math.min(Math.max(1, numberValue(tokens)), Math.max(1, numberValue(total)));
}

function renderTokenVolume(report) {
  const tokenVolume = document.querySelector("#token-volume");
  if (!tokenVolume) return;
  const metrics = report?.metrics || {};
  tokenVolume.replaceChildren();
  tokenVolume.append(
    `Analyzed token volume: ${formatNumber(numberValue(metrics.estimated_tokens))} estimated input/output tokens; ` +
      `${formatNumber(numberValue(metrics.tool_output_tokens))} estimated from tool output. `,
  );
  tokenVolume.appendChild(buildHelpTip(
    "Accuracy depends on the source log. When native usage fields exist, we use them. Otherwise we estimate roughly one token per four characters. Tool-output volume is derived from tool-result payload size and similar estimates. This is directional, not invoice-grade accounting.",
  ));
  window.AgentAnalyzerTooltips?.init(tokenVolume);
}

function buildHelpTip(text) {
  const tip = document.createElement("button");
  tip.type = "button";
  tip.className = "help-tip";
  tip.setAttribute("data-tippy-content", text);
  tip.setAttribute("aria-label", "More information");
  tip.textContent = "?";
  return tip;
}

function bubbleDiameter(tokens, maxTokens) {
  const ratio = Math.min(1, Math.max(0, numberValue(tokens) / Math.max(1, numberValue(maxTokens))));
  return 170 + Math.round(ratio * 98);
}

function bubbleLabelFontSize(text, diameter, maxPx, minPx) {
  const chars = Math.max(1, String(text || "").length);
  const available = Math.max(90, diameter * 0.72);
  const estimated = available / (chars * 0.56);
  return Number(Math.max(minPx, Math.min(maxPx, estimated)).toFixed(1));
}

function bubbleTone(finding, index) {
  switch (finding?.id) {
    case "tool_output_bloat":
    case "cache_invalidation_spike":
      return "orange";
    case "repeated_file_reads":
    case "context_growth_spikes":
      return "blue";
    case "retry_loop":
    case "args_hashed_retry_loop":
      return "green";
    default:
      return ["orange", "blue", "green"][index % 3];
  }
}

function bubbleOffset(index) {
  return [0, 28, -8, 18, -18, 10][index % 6];
}

function renderTimeline(points, estimatedWaste) {
  const chart = document.querySelector("#timeline");
  const yMax = document.querySelector("#timeline-y-max");
  const xAxis = document.querySelector("#timeline-x-axis");
  const legend = document.querySelector("#timeline-legend");
  chart.innerHTML = "";
  if (xAxis) xAxis.replaceChildren();
  if (legend) legend.replaceChildren();
  if (points.length === 0) {
    chart.textContent = "No timeline points detected.";
    chart.removeAttribute("aria-label");
    if (yMax) yMax.textContent = "max";
    return;
  }
  const visiblePoints = points.slice(-60);
  const maxTokens = Math.max(...visiblePoints.map((point) => numberValue(point.estimated_tokens)), 1);
  const wasteRange = normalizeWasteRange(estimatedWaste);
  const savingsPct = Math.min(95, Math.max(0, (wasteRange.low + wasteRange.high) / 2));
  const firstTurn = numberValue(visiblePoints[0]?.turn);
  const lastTurn = numberValue(visiblePoints[visiblePoints.length - 1]?.turn);
  chart.setAttribute(
    "aria-label",
    `Session timeline showing estimated context/token volume from turn ${firstTurn} to turn ${lastTurn}; maximum ${formatNumber(maxTokens)} estimated tokens; potential savings range ${wasteRange.low}-${wasteRange.high} percent.`,
  );
  if (yMax) yMax.textContent = `${formatCompactNumber(maxTokens)} tokens`;
  renderTimelineLegend(legend, wasteRange);
  for (const point of visiblePoints) {
    const bar = document.createElement("span");
    const estimatedTokens = numberValue(point.estimated_tokens);
    const savedTokensLow = Math.round(estimatedTokens * wasteRange.low / 100);
    const savedTokensHigh = Math.round(estimatedTokens * wasteRange.high / 100);
    const tooltip = [
      `turn ${numberValue(point.turn)}`,
      `${formatNumber(estimatedTokens)} estimated token volume`,
      `${formatNumber(savedTokensLow)}-${formatNumber(savedTokensHigh)} estimated potential savings`,
      `${formatNumber(numberValue(point.tool_tokens))} estimated tool-output tokens`,
      `${formatNumber(numberValue(point.rereads))} rereads`,
      `${formatNumber(numberValue(point.retries))} retries`,
    ].join(" | ");
    bar.className = "timeline-bar";
    bar.style.height = `${Math.max(4, (estimatedTokens / maxTokens) * 100)}%`;
    bar.dataset.tooltip = tooltip;
    bar.tabIndex = 0;
    bar.setAttribute("role", "img");
    bar.setAttribute("aria-label", tooltip);
    if (savingsPct > 0) {
      const savings = document.createElement("span");
      savings.className = "timeline-savings";
      savings.style.height = `${savingsPct}%`;
      savings.setAttribute("aria-hidden", "true");
      bar.appendChild(savings);
    }
    chart.appendChild(bar);
  }
  renderTimelineAxis(xAxis, visiblePoints);
}

function normalizeWasteRange(estimatedWaste) {
  const low = Math.round(clampPercent(estimatedWaste?.low));
  const high = Math.round(clampPercent(estimatedWaste?.high));
  return {
    low: Math.min(low, high),
    high: Math.max(low, high),
  };
}

function clampPercent(value) {
  return Math.min(100, Math.max(0, numberValue(value)));
}

function renderTimelineLegend(legend, wasteRange) {
  if (!legend) return;
  const observed = document.createElement("span");
  observed.className = "timeline-legend-item";
  const observedSwatch = document.createElement("span");
  observedSwatch.className = "timeline-legend-swatch timeline-legend-observed";
  observed.appendChild(observedSwatch);
  observed.append("estimated volume");
  legend.appendChild(observed);

  const avoidable = document.createElement("span");
  avoidable.className = "timeline-legend-item";
  const avoidableSwatch = document.createElement("span");
  avoidableSwatch.className = "timeline-legend-swatch timeline-legend-savings";
  avoidable.appendChild(avoidableSwatch);
  avoidable.append(`${wasteRange.low}-${wasteRange.high}% potential savings`);
  legend.appendChild(avoidable);
}

function renderTimelineAxis(axis, visiblePoints) {
  if (!axis || visiblePoints.length === 0) return;
  const first = visiblePoints[0];
  const middle = visiblePoints[Math.floor((visiblePoints.length - 1) / 2)];
  const last = visiblePoints[visiblePoints.length - 1];
  const ticks = [
    { key: "first", label: `turn ${numberValue(first.turn)}` },
    { key: "middle", label: `turn ${numberValue(middle.turn)}` },
    { key: "last", label: `turn ${numberValue(last.turn)}` },
  ].filter((tick, index, all) => all.findIndex((item) => item.label === tick.label) === index);
  for (const tick of ticks) {
    const item = document.createElement("span");
    item.className = `timeline-tick timeline-tick-${tick.key}`;
    item.textContent = tick.label;
    axis.appendChild(item);
  }
}

function numberValue(value) {
  return Number.isFinite(Number(value)) ? Number(value) : 0;
}

function formatNumber(value) {
  return new Intl.NumberFormat("en-US").format(numberValue(value));
}

function formatCompactNumber(value) {
  return new Intl.NumberFormat("en-US", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(numberValue(value));
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

// Token-saving recommendation rendering — kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4.
//
// Every text node is composed via textContent from allowlisted enum values
// produced by internal/analyzer/token_saving_*.go. Unknown values are never
// echoed into the DOM; that keeps this renderer privacy-safe even if it is
// handed malformed report JSON.

const TOOL_LABEL = {
  ccusage: "ccusage",
  ccstatusline: "ccstatusline",
  claude_code_usage_monitor: "Claude Code Usage Monitor",
  claude_code_usage_tracker: "Claude Code Usage Tracker",
  tokenusage: "tokenusage",
  claude_meter: "Claude Meter",
  context_mode: "Context Mode",
  distill: "Distill",
  token_optimizer_mcp: "Token Optimizer MCP",
  rtk: "RTK (Rust Token Killer, rtk-ai/rtk)",
  leanctx: "LeanCtx",
  headroom: "Headroom",
  claude_context: "Claude Context",
  grepai: "GrepAI",
  serena: "Serena",
  codegraph: "CodeGraph",
  codebase_memory_mcp: "Codebase Memory MCP",
  code_review_graph: "Code Review Graph",
  semble: "Semble",
  jcodemunch_mcp: "jcodemunch MCP",
  token_savior: "Token Savior",
  cocoindex_code: "CocoIndex Code",
  read_once: "Read Once",
  openwolf: "OpenWolf",
  memsearch: "MemSearch",
  claude_token_efficient: "Claude Token Efficient",
  caveman: "Caveman",
  claude_code_hooks_mastery: "Claude Code Hooks Mastery",
  awesome_claude_code: "Awesome Claude Code",
};

const TOOL_URL = {
  ccusage: "https://github.com/ryoppippi/ccusage",
  ccstatusline: "https://github.com/sirmalloc/ccstatusline",
  claude_code_usage_monitor: "https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor",
  claude_code_usage_tracker: "https://github.com/LyndonWangWork/Claude-Code-Usage-Tracker",
  context_mode: "https://github.com/mksglu/context-mode",
  rtk: "https://github.com/rtk-ai/rtk",
  claude_context: "https://github.com/zilliztech/claude-context",
  grepai: "https://github.com/yoanbernabeu/grepai",
  memsearch: "https://github.com/zilliztech/memsearch",
  claude_token_efficient: "https://github.com/drona23/claude-token-efficient",
  caveman: "https://github.com/JuliusBrussee/caveman",
  claude_code_hooks_mastery: "https://github.com/disler/claude-code-hooks-mastery",
  awesome_claude_code: "https://github.com/hesreallyhim/awesome-claude-code",
};

const REASON_LABEL = {
  absent: "Not detected yet",
  installed_inactive: "Installed but not active",
  configured_inactive: "Configured but not active",
  active_persistent: "Already active",
  rejected_alternative: "Previously rejected",
  prune_first: "Prune your current tooling first",
  audit_config: "Audit current config",
  no_op: "No action needed",
  server_quota_check: "Server quota check",
};

const CONFIDENCE_LABEL = {
  low: "Low confidence",
  medium: "Medium confidence",
  high: "High confidence",
};

const RISK_LABEL = {
  low: "Low risk",
  medium: "Medium risk",
  high: "High risk",
};

const POLICY_LABEL = {
  bundle: "Bundled",
  recommend: "Recommended",
  recommend_with_waiver: "Recommended (waiver required)",
  research_only: "Research only",
  reference_only: "Reference only",
};

const SIGNAL_LABEL = {
  tool_output_bloat: "Tool output bloat",
  shell_output_bloat: "Shell output bloat",
  mcp_tool_output_bloat: "MCP tool output bloat",
  repeated_file_reads: "Repeated file reads",
  broad_repo_exploration: "Broad repo exploration",
  unchanged_file_rereads: "Unchanged file rereads",
  mcp_skill_bloat: "MCP / skill bloat",
  output_verbosity: "Output verbosity",
  retry_loop: "Retry loop",
  context_growth_spikes: "Context growth spikes",
  no_usage_visibility: "No usage visibility",
};

const SAVINGS_BUCKET_LABEL = {
  low: "Low estimated savings",
  medium: "Medium estimated savings",
  high: "High estimated savings",
};

const FAILURE_MODE_LABEL = {
  noisy_terminal_logs: "Noisy terminal logs",
  tool_output_flooding: "Tool output flooding",
  repeated_codebase_navigation: "Repeated codebase navigation",
  broad_file_reads_or_verbose_output: "Broad reads / verbose output",
  memory_gaps: "Memory gaps",
  cross_cutting_telemetry: "Cross-cutting hygiene",
};

const INSTALL_SURFACE_LABEL = {
  local_binary_plus_claude_hook: "Local binary + Claude hook",
  claude_plugin_plus_mcp: "Claude plugin + MCP",
  mcp_plus_external_vector_store: "MCP + external vector store",
  local_binary_plus_optional_embedding_provider: "Local binary + optional embeddings",
  local_cli_or_local_config: "Local CLI/config",
  mcp_server: "MCP server",
  retrieval_tool: "Retrieval tool",
  local_instruction_config: "Local instruction config",
  prune_or_lazy_load_existing_mcp_and_skills: "Prune / lazy-load existing tools",
  session_workflow_and_config_audit: "Session workflow audit",
};

function savingsBucket(report) {
  const high = report?.estimated_waste_pct?.high ?? 0;
  if (high < 10) return "low";
  if (high < 30) return "medium";
  return "high";
}

function labelFrom(table, value, fallback) {
  return typeof value === "string" && Object.hasOwn(table, value) ? table[value] : fallback;
}

function renderRecommendation(report) {
  const section = document.querySelector("#recommendation-section");
  const primaryRoot = document.querySelector("#recommendation-primary");
  const secondaryRoot = document.querySelector("#recommendation-secondary");
  const emptyNote = document.querySelector("#recommendation-empty");
  if (!section || !primaryRoot || !secondaryRoot || !emptyNote) return;

  // Reset slot DOM on every render.
  primaryRoot.replaceChildren();
  secondaryRoot.replaceChildren();
  emptyNote.replaceChildren();

  // FR-012: legacy report JSON (no recommendation field) renders nothing.
  if (report?.recommendation == null) {
    section.hidden = true;
    primaryRoot.hidden = true;
    secondaryRoot.hidden = true;
    emptyNote.hidden = true;
    return;
  }

  const rec = report.recommendation;
  section.hidden = false;

  const primary = rec.primary;
  if (primary && typeof primary === "object") {
    primaryRoot.hidden = false;
    primaryRoot.appendChild(buildRecommendationCard(primary, savingsBucket(report)));
  } else {
    primaryRoot.hidden = true;
  }

  const secondary = rec.secondary;
  if (secondary && typeof secondary === "object") {
    secondaryRoot.hidden = false;
    secondaryRoot.appendChild(buildRecommendationCard(secondary, null));
  } else {
    secondaryRoot.hidden = true;
  }

  // FR-006 / FR-009: no-op note when both Primary and Secondary are absent.
  const noActionable = !primary && !secondary;
  if (noActionable) {
    const skippedCount = Array.isArray(rec.skipped) ? rec.skipped.length : 0;
    const unknownCount = typeof rec.unknown_id_count === "number" ? rec.unknown_id_count : 0;
    const sentence = document.createElement("span");
    let text =
      `Engine evaluated ${skippedCount} candidate${skippedCount === 1 ? "" : "s"}; ` +
      `none warranted a recommendation.`;
    if (unknownCount > 0) {
      text +=
        ` (${unknownCount} unknown identifier${unknownCount === 1 ? "" : "s"} ` +
        `${unknownCount === 1 ? "was" : "were"} counted only.)`;
    }
    sentence.textContent = text;
    emptyNote.appendChild(sentence);
    emptyNote.hidden = false;
  } else {
    emptyNote.hidden = true;
  }
}

function buildRecommendationCard(rec, savingsBucketValue) {
  const card = document.createElement("div");
  card.className = "recommendation-card";

  // Tool label. Advisory recommendations intentionally carry an empty
  // PrimaryToolID; render those as actions instead of blank tool cards.
  const toolID = typeof rec.primary_tool_id === "string" ? rec.primary_tool_id : "";
  const toolName = typeof rec.primary_tool_name === "string" ? rec.primary_tool_name : "";
  const toolEl = document.createElement("div");
  toolEl.className = "recommendation-tool";
  toolEl.textContent = toolID.length > 0
    ? (toolName.length > 0 ? toolName : labelFrom(TOOL_LABEL, toolID, "Unknown tool"))
    : advisoryRecommendationLabel(rec);
  card.appendChild(toolEl);

  const reportSourceURL = typeof rec.primary_tool_url === "string" ? rec.primary_tool_url : "";
  const sourceURL = reportSourceURL.startsWith("https://")
    ? reportSourceURL
    : (toolID.length > 0 && Object.hasOwn(TOOL_URL, toolID) ? TOOL_URL[toolID] : "");
  if (sourceURL.length > 0) {
    const source = document.createElement("a");
    source.className = "recommendation-source";
    source.href = sourceURL;
    source.rel = "noopener noreferrer";
    source.target = "_blank";
    source.textContent = sourceURL;
    card.appendChild(source);
  }

  // Optional savings-bucket badge (Primary only).
  if (typeof savingsBucketValue === "string" && savingsBucketValue.length > 0) {
    const savings = document.createElement("span");
    savings.className = "recommendation-savings-bucket";
    savings.textContent = labelFrom(SAVINGS_BUCKET_LABEL, savingsBucketValue, "Estimated savings");
    card.appendChild(savings);
  }

  const meta = document.createElement("div");
  meta.className = "recommendation-meta";

  const reason = typeof rec.reason === "string" ? rec.reason : "";
  const reasonEl = document.createElement("span");
  reasonEl.className = "recommendation-reason";
  reasonEl.textContent = labelFrom(REASON_LABEL, reason, "Unknown reason");
  meta.appendChild(reasonEl);

  const confidence = typeof rec.confidence === "string" ? rec.confidence : "";
  const confidenceEl = document.createElement("span");
  confidenceEl.className = "recommendation-confidence";
  confidenceEl.textContent = labelFrom(CONFIDENCE_LABEL, confidence, "Unknown confidence");
  meta.appendChild(confidenceEl);

  const risk = typeof rec.risk_level === "string" ? rec.risk_level : "";
  const riskEl = document.createElement("span");
  riskEl.className = "recommendation-risk";
  riskEl.textContent = labelFrom(RISK_LABEL, risk, "Unknown risk");
  meta.appendChild(riskEl);

  const policy = typeof rec.install_policy === "string" ? rec.install_policy : "";
  const policyEl = document.createElement("span");
  policyEl.className = "recommendation-policy";
  policyEl.textContent = labelFrom(POLICY_LABEL, policy, "Unknown policy");
  meta.appendChild(policyEl);

  card.appendChild(meta);

  const failureModes = Array.isArray(rec.failure_modes) ? rec.failure_modes : [];
  if (failureModes.length > 0 || typeof rec.install_surface === "string" || typeof rec.data_movement_risk === "string") {
    const fit = document.createElement("div");
    fit.className = "recommendation-fit";
    for (const mode of failureModes) {
      const id = typeof mode === "string" ? mode : "";
      if (!Object.hasOwn(FAILURE_MODE_LABEL, id)) continue;
      const chip = document.createElement("span");
      chip.textContent = FAILURE_MODE_LABEL[id];
      fit.appendChild(chip);
    }
    const surface = typeof rec.install_surface === "string" ? rec.install_surface : "";
    if (surface.length > 0) {
      const chip = document.createElement("span");
      chip.textContent = labelFrom(INSTALL_SURFACE_LABEL, surface, "Install surface reviewed");
      fit.appendChild(chip);
    }
    const dataRisk = typeof rec.data_movement_risk === "string" ? rec.data_movement_risk : "";
    if (Object.hasOwn(RISK_LABEL, dataRisk)) {
      const chip = document.createElement("span");
      chip.textContent = `Data movement: ${RISK_LABEL[dataRisk].toLowerCase()}`;
      fit.appendChild(chip);
    }
    if (fit.childElementCount > 0) card.appendChild(fit);
  }

  const conflicts = Array.isArray(rec.conflicts_with) ? rec.conflicts_with : [];
  const safeConflicts = conflicts
    .map((id) => (typeof id === "string" && Object.hasOwn(TOOL_LABEL, id) ? TOOL_LABEL[id] : ""))
    .filter(Boolean);
  if (safeConflicts.length > 0) {
    const conflict = document.createElement("p");
    conflict.className = "recommendation-conflicts";
    conflict.textContent = `Overlaps with ${safeConflicts.join(", ")}. Choose one tool for this failure mode unless you explicitly approve both.`;
    card.appendChild(conflict);
  }

  const ambiguity = typeof rec.ambiguity_warning === "string" ? rec.ambiguity_warning : "";
  if (ambiguity.length > 0 && toolID === "rtk") {
    const warning = document.createElement("p");
    warning.className = "recommendation-warning";
    warning.textContent = "RTK means github.com/rtk-ai/rtk. Do not install the unrelated npm package named rtk.";
    card.appendChild(warning);
  }

  // Signal chips.
  const signals = Array.isArray(rec.signal_ids) ? rec.signal_ids : [];
  if (signals.length > 0) {
    const signalList = document.createElement("ul");
    signalList.className = "recommendation-signals";
    for (const sig of signals) {
      const id = typeof sig === "string" ? sig : "";
      if (id.length === 0) continue;
      const chip = document.createElement("li");
      chip.className = "recommendation-signal";
      chip.textContent = labelFrom(SIGNAL_LABEL, id, "Unknown signal");
      signalList.appendChild(chip);
    }
    card.appendChild(signalList);
  }

  return card;
}

function advisoryRecommendationLabel(rec) {
  const reason = typeof rec.reason === "string" ? rec.reason : "";
  const signals = Array.isArray(rec.signal_ids) ? rec.signal_ids : [];
  if (reason === "prune_first" || signals.includes("mcp_skill_bloat")) {
    return "Prune / lazy-load MCPs and skills";
  }
  if (reason === "audit_config" || signals.includes("retry_loop") || signals.includes("context_growth_spikes")) {
    return "Session hygiene audit";
  }
  return "Tooling recommendation";
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

function renderEcosystem(ecosystem) {
  const target = document.querySelector("#ecosystem");
  if (!target) return;
  target.textContent = "";
  if (!ecosystem) {
    target.appendChild(emptyPanel("No ecosystem signals detected."));
    return;
  }
  const summary = document.createElement("div");
  summary.className = "ecosystem-summary";
  [
    ["Client", ecosystem.client || "unknown"],
    ["OS", ecosystem.operating_system || "unknown"],
    ["Shell", ecosystem.shell || "unknown"],
    ["Version control", ecosystem.version_control || "unknown"],
  ].forEach(([label, value]) => summary.appendChild(metricPill(label, value)));
  target.appendChild(summary);

  const groups = document.createElement("div");
  groups.className = "evidence-groups";
  groups.appendChild(chipGroup("Coding agents", ecosystem.coding_agents));
  groups.appendChild(chipGroup("Workflow frameworks", ecosystem.workflow_frameworks));
  groups.appendChild(chipGroup("MCPs", ecosystem.mcp_servers_known, `${numberOrZero(ecosystem.unknown_mcp_server_count)} unknown`));
  groups.appendChild(chipGroup("Skills", ecosystem.known_skills, `${numberOrZero(ecosystem.unknown_skill_count)} unknown`));
  groups.appendChild(chipGroup("Plugins", ecosystem.known_plugins, `${numberOrZero(ecosystem.unknown_plugin_count)} unknown`));
  groups.appendChild(chipGroup("Package managers", ecosystem.package_managers));
  target.appendChild(groups);
}

function renderReceipt(receipt, redactions) {
  const target = document.querySelector("#receipt");
  if (!target) return;
  target.textContent = "";
  if (!receipt) {
    target.appendChild(emptyPanel("No security receipt available."));
    return;
  }
  const statusGrid = document.createElement("div");
  statusGrid.className = "receipt-status-grid";
  statusGrid.appendChild(statusTile("Raw transcript to LLM", receipt.raw_transcript_sent_to_llm === true ? "yes" : "no", receipt.raw_transcript_sent_to_llm === true ? "bad" : "good"));
  statusGrid.appendChild(statusTile("Outbound during analysis", receipt.outbound_during_analysis === true ? "yes" : "no", receipt.outbound_during_analysis === true ? "bad" : "good"));
  statusGrid.appendChild(statusTile("Raw log TTL", receipt.raw_log_ttl || "unknown", receipt.raw_log_ttl === "not uploaded" ? "good" : "warn"));
  statusGrid.appendChild(statusTile("Secrets redacted", String(numberOrZero(receipt.secrets_redacted)), numberOrZero(receipt.secrets_redacted) > 0 ? "warn" : "good"));
  target.appendChild(statusGrid);
  target.appendChild(redactionGroup(redactions || receipt.redactions));
}

function metricPill(label, value) {
  const item = document.createElement("span");
  item.className = "metric-pill";
  const k = document.createElement("small");
  k.textContent = label;
  const v = document.createElement("strong");
  v.textContent = value || "unknown";
  item.append(k, v);
  return item;
}

function chipGroup(label, values, extra) {
  const group = document.createElement("section");
  group.className = "chip-group";
  const title = document.createElement("h3");
  title.textContent = label;
  group.appendChild(title);
  const list = document.createElement("div");
  list.className = "chip-list";
  const safeValues = Array.isArray(values) ? values.filter(Boolean) : [];
  if (safeValues.length === 0) {
    list.appendChild(chip("none detected", "muted"));
  } else {
    safeValues.forEach((value) => list.appendChild(chip(String(value), "")));
  }
  if (extra && !String(extra).startsWith("0 ")) {
    list.appendChild(chip(extra, "unknown"));
  }
  group.appendChild(list);
  return group;
}

function chip(text, tone) {
  const item = document.createElement("span");
  item.className = `info-chip${tone ? ` info-chip-${tone}` : ""}`;
  item.textContent = text;
  return item;
}

function statusTile(label, value, tone) {
  const item = document.createElement("div");
  item.className = `receipt-tile receipt-tile-${tone}`;
  item.setAttribute("aria-label", `${label}: ${value}`);
  const k = document.createElement("small");
  k.textContent = label;
  const v = document.createElement("strong");
  v.textContent = value;
  item.append(k, v);
  return item;
}

function redactionGroup(redactions) {
  const group = document.createElement("section");
  group.className = "chip-group redaction-group";
  const title = document.createElement("h3");
  title.textContent = "Redactions";
  const list = document.createElement("div");
  list.className = "chip-list";
  const entries = Object.entries(redactions || {}).filter(([, value]) => Number(value) > 0);
  if (entries.length === 0) {
    list.appendChild(chip("none detected", "muted"));
  } else {
    entries.forEach(([key, value]) => list.appendChild(chip(`${key}: ${value}`, "unknown")));
  }
  group.append(title, list);
  return group;
}

function emptyPanel(text) {
  const p = document.createElement("p");
  p.className = "empty-evidence";
  p.textContent = text;
  return p;
}

function numberOrZero(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
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
  if ((report?.aggregate_event?.parser_type === "paid_bundle" || report?.aggregate_event?.parser_type === "full_scan_bundle") && route) {
    const pluginURL = `${window.location.origin}/api/public-artifacts/${route.jobID}/${route.token}/plugin.zip`;
    const command = [
      `PLUGIN_URL="${pluginURL}"`,
      'PLUGIN_ZIP="$(mktemp -t agent-analyzer-plugin.XXXXXX.zip)"',
      'curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"',
      'claude --plugin-dir "$PLUGIN_ZIP"',
    ].join("\n");
    target.textContent =
      "Install the generated Agent Analyzer optimization plugin for this session. Run the command below, explain what it installs, summarize the waiver, and ask for approval before executing it. Do not print plugin archive contents.\n\n```sh\n" +
      command +
      "\n```";
    if (paidStatus) paidStatus.textContent = "optimization plugin ready; artifact URL expires with this report";
    if (copyPaidCommandButton) copyPaidCommandButton.hidden = false;
    if (unlockPaidButton) unlockPaidButton.hidden = true;
    if (waiverAccepted) waiverAccepted.closest("label").hidden = true;
    const upsellCopy = document.querySelectorAll(".upsell p");
    if (upsellCopy[0]) {
      upsellCopy[0].textContent =
        "Your full scan is complete. The optimization plugin below is generated from sanitized aggregate findings and vetted tooling recommendations.";
    }
    if (upsellCopy[1]) {
      upsellCopy[1].textContent =
        "Review the install command with Claude Code. Claude should summarize the waiver and ask before running it.";
    }
    return;
  }
  target.textContent = "Confirm your email to get the full-scan NPX command.";
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
