const form = document.querySelector("#upload-form");
const statusEl = document.querySelector("#status");
const reportEl = document.querySelector("#report");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = new FormData(form);
  statusEl.textContent = "uploading";
  reportEl.hidden = true;

  const response = await fetch("/api/jobs", { method: "POST", body: data });
  if (!response.ok) {
    statusEl.textContent = `upload failed: ${await response.text()}`;
    return;
  }
  const job = await response.json();
  statusEl.textContent = `queued ${job.job_id}`;
  await poll(job.job_id);
});

async function poll(jobID) {
  for (;;) {
    const response = await fetch(`/api/jobs/${jobID}`);
    const job = await response.json();
    statusEl.textContent = JSON.stringify(job, null, 2);
    if (job.status === "completed") {
      const reportResponse = await fetch(`/api/reports/${jobID}`);
      renderReport(await reportResponse.json());
      return;
    }
    if (job.status === "failed") {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

function renderReport(report) {
  document.querySelector("#score").textContent = report.score;
  document.querySelector("#waste").textContent =
    `${report.estimated_waste_pct.low}-${report.estimated_waste_pct.high}% avoidable token spend`;

  const findings = document.querySelector("#findings");
  findings.innerHTML = "";
  for (const finding of report.findings) {
    const item = document.createElement("li");
    item.textContent = `${finding.title} (${finding.severity}): ${finding.recommendation}`;
    findings.appendChild(item);
  }
  if (report.findings.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No major deterministic waste pattern detected.";
    findings.appendChild(item);
  }
  document.querySelector("#ecosystem").textContent = JSON.stringify(report.ecosystem, null, 2);
  document.querySelector("#receipt").textContent = JSON.stringify(report.security_receipt, null, 2);
  reportEl.hidden = false;
}

