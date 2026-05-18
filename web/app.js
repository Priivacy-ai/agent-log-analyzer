const form = document.querySelector("#upload-form");
const statusEl = document.querySelector("#status");
const reportEl = document.querySelector("#report");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const file = form.querySelector('input[type="file"]').files[0];
  if (!file) {
    statusEl.textContent = "select a log file";
    return;
  }
  statusEl.textContent = "uploading";
  reportEl.hidden = true;

  try {
    const job = await uploadWithBestAvailablePath(file);
    statusEl.textContent = `queued ${job.job_id}`;
    await poll(job.job_id);
  } catch (error) {
    statusEl.textContent = `upload failed: ${error.message}`;
  }
});

async function uploadWithBestAvailablePath(file) {
  try {
    return await uploadDirect(file);
  } catch (error) {
    if (error.status !== 404 && error.status !== 501) {
      throw error;
    }
    return uploadMultipart();
  }
}

async function uploadDirect(file) {
  const initResponse = await retryFetch(() => fetch("/api/upload-url", { method: "POST" }));
  if (!initResponse.ok) {
    throw await responseError(initResponse);
  }
  const upload = await initResponse.json();
  let uploadResponse;
  if (upload.fields && Object.keys(upload.fields).length > 0) {
    uploadResponse = await retryFetch(() => {
      const body = new FormData();
      for (const [key, value] of Object.entries(upload.fields)) {
        body.append(key, value);
      }
      body.append("file", file);
      return fetch(upload.url, { method: upload.method, body });
    });
  } else {
    uploadResponse = await retryFetch(() => fetch(upload.url, {
      method: upload.method,
      headers: upload.headers || {},
      body: file,
    }));
  }
  if (!uploadResponse.ok) {
    throw new Error(`direct upload failed: ${uploadResponse.status}`);
  }
  const finalizeResponse = await retryFetch(() => fetch(upload.finalize_path, { method: "POST" }));
  if (!finalizeResponse.ok) {
    throw await responseError(finalizeResponse);
  }
  return finalizeResponse.json();
}

async function uploadMultipart() {
  const data = new FormData(form);
  const response = await fetch("/api/jobs", { method: "POST", body: data });
  if (!response.ok) {
    throw await responseError(response);
  }
  return response.json();
}

async function responseError(response) {
  const error = new Error(await response.text());
  error.status = response.status;
  return error;
}

async function retryFetch(operation, attempts = 3) {
  let lastError;
  for (let attempt = 0; attempt < attempts; attempt++) {
    try {
      const response = await operation();
      if (response.status < 500 && response.status !== 429) {
        return response;
      }
      lastError = new Error(`status ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    if (attempt + 1 < attempts) {
      await new Promise((resolve) => setTimeout(resolve, 250 * 2 ** attempt));
    }
  }
  throw lastError;
}

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
