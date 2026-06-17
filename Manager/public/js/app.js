const uploadState = {
  scanId: null,
  linksource: null,
  linkrawswagger: null,
};

// ---------- Role / access ----------
let currentRole = "guest"; // "guest" | "admin"

async function detectRole() {
  try {
    const res = await fetch("/api/me");
    if (res.ok) {
      const data = await res.json();
      currentRole = data.role === "admin" ? "admin" : "guest";
    }
  } catch (_) {}
  document.documentElement.classList.toggle("is-admin", currentRole === "admin");
}

// ---------- Models page state (shared between loadModelsPage + validateModelDropdowns) ----------
let modelsPageState = {
  modelNames: [],
  remoteById: {},
  assignmentMap: {},
  connected: false,
};

const API_URL_STORAGE_KEY = "agentsdast.apiUrl.value";
const API_URL_REMEMBER_KEY = "agentsdast.apiUrl.remember";

// Thin wrapper over the shared toast component (UI.toast) so all existing
// callers keep working. opts may carry { details } for an expandable + copyable
// technical block on error toasts.
function showToast(message, type = "success", opts = {}) {
  const o = typeof opts === "number" ? { duration: opts } : opts || {};
  return UI.toast.show(message, { type, ...o });
}

function isApiUrlRememberEnabled() {
  return localStorage.getItem(API_URL_REMEMBER_KEY) === "1";
}

function restoreRememberedApiUrl() {
  const input = document.getElementById("api-url");
  const remember = document.getElementById("api-url-remember");
  if (!input || !remember) return;

  const enabled = isApiUrlRememberEnabled();
  remember.checked = enabled;
  if (enabled) {
    const saved = localStorage.getItem(API_URL_STORAGE_KEY);
    if (saved) input.value = saved;
  }
}

function persistApiUrlIfRemembered() {
  const input = document.getElementById("api-url");
  const remember = document.getElementById("api-url-remember");
  if (!input || !remember) return;

  if (remember.checked) {
    localStorage.setItem(API_URL_REMEMBER_KEY, "1");
    localStorage.setItem(API_URL_STORAGE_KEY, input.value.trim());
  } else {
    localStorage.removeItem(API_URL_REMEMBER_KEY);
    localStorage.removeItem(API_URL_STORAGE_KEY);
  }
}

function applyRememberedApiUrlToInput() {
  if (!isApiUrlRememberEnabled()) return;
  const saved = localStorage.getItem(API_URL_STORAGE_KEY);
  if (saved) document.getElementById("api-url").value = saved;
}

const ADMIN_PAGES = ["models", "skills"];

function navigate(page) {
  // Redirect guests away from admin-only pages
  if (ADMIN_PAGES.includes(page) && currentRole !== "admin") {
    page = "dashboard";
  }

  document.querySelectorAll(".view").forEach((v) => v.classList.remove("active"));
  document.querySelectorAll(".nav-link").forEach((l) => l.classList.remove("active"));

  const view = document.getElementById(`view-${page}`);
  const link = document.querySelector(`.nav-link[data-page="${page}"]`);
  if (view) view.classList.add("active");
  if (link) link.classList.add("active");

  if (page === "dashboard") loadDashboard();
  if (page === "scan") {
    restoreRememberedApiUrl();
    loadScanHistory();
  }
  if (page === "models") {
    stopScanPolling();
    loadModelsPage();
  }
  if (page === "skills") {
    stopScanPolling();
    loadSkillsPage().then(() => {
      if (window.SkillsEditor) SkillsEditor.refresh();
    });
  }
  if (page !== "scan") stopScanPolling();

  history.replaceState(null, "", `#/${page}`);
}

const VALID_PAGES = ["dashboard", "scan", "models", "skills"];

function resolvePage(hash) {
  const page = (hash || "dashboard").replace("#/", "");
  return VALID_PAGES.includes(page) ? page : "dashboard";
}

async function initRouter() {
  // Detect role before first navigation so admin pages aren't incorrectly redirected.
  await detectRole();
  navigate(resolvePage(location.hash));

  document.querySelectorAll(".nav-link").forEach((link) => {
    link.addEventListener("click", (e) => {
      e.preventDefault();
      navigate(link.dataset.page);
    });
  });

  window.addEventListener("hashchange", () => {
    navigate(resolvePage(location.hash));
  });
}

function badge(status) {
  const s = (status || "new").toLowerCase();
  return `<span class="badge ${s}">${s}</span>`;
}

// selectedScanModes reads the exclusive Quick/Deep radio + the PGW add-on.
// Quick and Deep are mutually exclusive (radio); PGW is independent.
function selectedScanModes() {
  const modes = [];
  modes.push(document.getElementById("mode-deepscan")?.checked ? "deepscan" : "quickscan");
  if (document.getElementById("mode-pgwscan")?.checked) modes.push("pgwscan");
  return modes;
}

const SCAN_MODE_LABELS = { quickscan: "Quick", deepscan: "Deep", pgwscan: "PGW" };

// shortModelLabel strips the provider prefix from a full model ID.
// "minimax/minimax-m2.5" → "minimax-m2.5", "gpt-4o" → "gpt-4o"
function shortModelLabel(fullName) {
  if (!fullName) return null;
  const idx = fullName.indexOf("/");
  return idx !== -1 ? fullName.slice(idx + 1) : fullName;
}

// modelChip renders a compact badge. Pass null/undefined fullModelName to
// get an "unknown" chip for completed scans where the model wasn't recorded.
function modelChip(agentKey, fullModelName) {
  if (!fullModelName) {
    return `<span class="model-chip model-chip-unknown" title="Model not recorded">unknown</span>`;
  }
  const label = shortModelLabel(fullModelName);
  return `<span class="model-chip model-chip-${agentKey}" title="${escHtml(fullModelName)}">${escHtml(label)}</span>`;
}

// renderModelChipRow always renders the chip container to keep SAST/DAST columns
// vertically aligned. Shows the model chip if known, or an "unknown" muted chip.
function renderModelChipRow(agentKey, fullModelName) {
  return `<div class="row-model">${modelChip(agentKey, fullModelName)}</div>`;
}

// formatScanModes renders a CSV/array of modes as small badges.
function formatScanModes(modes) {
  const arr = Array.isArray(modes)
    ? modes
    : String(modes || "").split(",").map((s) => s.trim()).filter(Boolean);
  if (!arr.length) return "";
  return arr
    .map((m) => `<span class="mode-badge mode-${m}">${SCAN_MODE_LABELS[m] || m}</span>`)
    .join(" ");
}

function setHealthCard(prefix, health, meta, modelName) {
  const dot = document.getElementById(`${prefix}-health-dot`);
  const label = document.getElementById(`${prefix}-health-label`);
  const metaEl = document.getElementById(`${prefix}-health-meta`);
  if (dot) dot.className = `health-dot health-${health}`;
  if (label) label.textContent = health;
  if (metaEl) metaEl.textContent = meta;
  if (modelName !== undefined) {
    const modelEl = document.getElementById(`${prefix}-model-name`);
    if (modelEl) modelEl.textContent = modelName || "—";
  }
}

function infraHealthLevel(status) {
  if (status === "up") return "healthy";
  if (status === "degraded") return "degraded";
  return "critical";
}

function agentDisplayHealth(agent) {
  if (agent.service?.status !== "up") return "critical";
  const h = agent.health || "healthy";
  if (h === "idle") return "healthy";
  if (h === "progress") return "running";
  return h;
}

async function loadDashboard() {
  try {
    const res = await fetch("/api/dashboard/summary");
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    document.getElementById("stat-total").textContent = data.scans.total;
    document.getElementById("stat-new").textContent = data.scans.new;
    document.getElementById("stat-progress").textContent = data.scans.progress;
    document.getElementById("stat-done").textContent = data.scans.done;
    document.getElementById("stat-fail").textContent = data.scans.fail;
    document.getElementById("stat-cancel").textContent = data.scans.cancel;

    const sast = data.agents.sast;
    const dast = data.agents.dast;
    const report = data.agents.report;
    setHealthCard(
      "sast",
      agentDisplayHealth(sast),
      `${sast.total} jobs · ${sast.progress} progress · service: ${sast.service?.status || "unknown"}`,
      data.model_assignments?.sast
    );
    setHealthCard(
      "dast",
      agentDisplayHealth(dast),
      `${dast.total} jobs · ${dast.progress} progress · service: ${dast.service?.status || "unknown"}`,
      data.model_assignments?.dast
    );
    if (report) {
      setHealthCard(
        "report",
        agentDisplayHealth(report),
        `${report.total} jobs · ${report.progress} progress · service: ${report.service?.status || "unknown"}`,
        data.model_assignments?.report
      );
    }

    const mysql = data.mysql || data.infrastructure?.mysql;
    if (mysql) {
      setHealthCard(
        "mysql",
        infraHealthLevel(mysql.status),
        mysql.message || `${mysql.host || "—"}:${mysql.port || "—"}`
      );
    }

    const minio = data.minio || data.infrastructure?.minio;
    if (minio) {
      setHealthCard(
        "minio",
        infraHealthLevel(minio.status),
        minio.message || `${minio.endpoint || "—"} · ${minio.bucket || "—"}`
      );
    }

    if (data.db_error) {
      showToast(`Database: ${data.db_error}`, "error");
    }

    setHealthCard(
      "llm",
      data.llm.status === "up" ? "healthy" : "critical",
      data.llm.connected ? `${data.llm.message}` : `${data.llm.message || "not connected"}`
    );

    const breakdownEl = document.getElementById("status-breakdown");
    if (!data.status_breakdown.length) {
      breakdownEl.innerHTML = '<tr><td colspan="2" class="empty">No data</td></tr>';
    } else {
      breakdownEl.innerHTML = data.status_breakdown
        .map((row) => `<tr><td>${badge(row.status)}</td><td>${row.count}</td></tr>`)
        .join("");
    }

    const recentEl = document.getElementById("recent-scans");
    if (!data.recent_scans.length) {
      recentEl.innerHTML = '<tr><td colspan="5" class="empty">No scans yet</td></tr>';
    } else {
      recentEl.innerHTML = data.recent_scans
        .map(
          (r) => `
        <tr>
          <td title="${r.id}">${formatScanId(r.id)}</td>
          <td>${badge(r.status)}</td>
          <td>${r.linkapi}</td>
          <td>${r.sastid ? "Yes" : "—"}</td>
          <td>${new Date(r.created_at).toLocaleString()}</td>
        </tr>`
        )
        .join("");
    }
  } catch (err) {
    showToast(err.message || "Failed to load dashboard", "error");
  }
}

function formatScanId(id) {
  if (!id) return "—";
  return id.length > 13 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function setDropzonesEnabled(enabled) {
  document.querySelectorAll(".dropzone").forEach((zone) => {
    zone.classList.toggle("disabled", !enabled);
  });
}

function updateCreateButton() {
  const apiUrl = document.getElementById("api-url");
  const btn = document.getElementById("btn-create");
  const ready =
    uploadState.scanId &&
    uploadState.linksource &&
    uploadState.linkrawswagger &&
    apiUrl.value.trim().length > 0;
  btn.disabled = !ready;
}

// Reveal the New Scan form (after init or resume) and focus the first input.
function expandNewScan() {
  document.getElementById("new-scan-collapsed").hidden = true;
  document.getElementById("new-scan-form").hidden = false;
  document.getElementById("btn-collapse-scan").hidden = false;
  const focusEl = document.getElementById("api-url");
  if (focusEl) setTimeout(() => focusEl.focus(), 0);
}

// Collapse back to the default state (just the Init/Resume button).
function collapseNewScan() {
  document.getElementById("new-scan-form").hidden = true;
  document.getElementById("new-scan-collapsed").hidden = false;
  document.getElementById("btn-collapse-scan").hidden = true;
  // With an active session the button resumes the form; otherwise it inits fresh.
  document.getElementById("btn-init-scan-label").textContent = uploadState.scanId
    ? "Resume Scan"
    : "Init Scan";
}

async function initScanSession() {
  const btn = document.getElementById("btn-init-scan");
  const prevHtml = btn.innerHTML;
  btn.disabled = true;
  btn.textContent = "Initializing...";

  try {
    const res = await fetch("/api/scan/init", { method: "POST" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    uploadState.scanId = data.id;
    uploadState.linksource = null;
    uploadState.linkrawswagger = null;
    document.getElementById("api-url").value = "";
    applyRememberedApiUrlToInput();
    document.getElementById("source-filename").textContent = "";
    document.getElementById("swagger-filename").textContent = "";
    document.querySelectorAll(".dropzone").forEach((z) => z.classList.remove("done"));

    document.getElementById("scan-id-label").textContent = `// scan_id = ${data.id}`;
    setDropzonesEnabled(true);
    updateCreateButton();
    expandNewScan();
    showToast("Scan initialized");
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    btn.disabled = false;
    btn.innerHTML = prevHtml;
  }
}

function canonicalUploadName(file, type) {
  if (type === "source") return "source_code.zip";
  const name = (file.name || "").toLowerCase();
  if (name.endsWith(".json")) return "raw_swagger.json";
  if (name.endsWith(".yaml")) return "raw_swagger.yaml";
  if (name.endsWith(".yml")) return "raw_swagger.yml";
  return file.name;
}

const MAX_UPLOAD_BYTES = 200 * 1024 * 1024; // 200 MB — must match backend + nginx

async function uploadFile(file, type) {
  if (!uploadState.scanId) {
    throw new Error("Initialize scan first");
  }
  // Client-side guard so oversized files get a clear message without a round-trip.
  if (file.size > MAX_UPLOAD_BYTES) {
    throw new Error(
      `File is ${(file.size / 1048576).toFixed(1)} MB — maximum upload size is 200 MB.`
    );
  }
  const endpoint =
    type === "source"
      ? `/api/upload/${uploadState.scanId}/source`
      : `/api/upload/${uploadState.scanId}/swagger`;
  const formData = new FormData();
  // Server stores canonical MinIO keys; original name is only used for extension checks.
  formData.append("file", file, canonicalUploadName(file, type));

  const res = await fetch(endpoint, { method: "POST", body: formData });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Upload failed");
  return data;
}

function setupDropzones() {
  document.querySelectorAll(".dropzone").forEach((zone) => {
    const type = zone.dataset.type;
    const fileInput = zone.querySelector('input[type="file"]');
    const filenameEl = document.getElementById(`${type}-filename`);

    async function handleFile(file) {
      if (!file || !uploadState.scanId) return;

      zone.classList.add("uploading");
      filenameEl.textContent = "Uploading...";

      try {
        const result = await uploadFile(file, type);
        if (type === "source") uploadState.linksource = result.path;
        else uploadState.linkrawswagger = result.path;

        filenameEl.textContent = result.filename;
        zone.classList.remove("uploading");
        zone.classList.add("done");
        updateCreateButton();
      } catch (err) {
        zone.classList.remove("uploading");
        filenameEl.textContent = "";
        showToast(err.message, "error");
      }
    }

    fileInput.addEventListener("change", (e) => handleFile(e.target.files[0]));
    zone.addEventListener("dragover", (e) => {
      e.preventDefault();
      zone.classList.add("dragover");
    });
    zone.addEventListener("dragleave", () => zone.classList.remove("dragover"));
    zone.addEventListener("drop", (e) => {
      e.preventDefault();
      zone.classList.remove("dragover");
      handleFile(e.dataTransfer.files[0]);
    });
  });

  document.getElementById("api-url").addEventListener("input", () => {
    persistApiUrlIfRemembered();
    updateCreateButton();
  });

  document.getElementById("api-url-remember").addEventListener("change", () => {
    persistApiUrlIfRemembered();
  });

  document.getElementById("btn-check-api-url").addEventListener("click", checkApiUrlConnect);

  document.getElementById("btn-init-scan").addEventListener("click", () => {
    // Resume the form if a session already exists; otherwise create a new record.
    if (uploadState.scanId) expandNewScan();
    else initScanSession();
  });

  document.getElementById("btn-collapse-scan").addEventListener("click", collapseNewScan);

  document.getElementById("btn-create").addEventListener("click", async () => {
    const btn = document.getElementById("btn-create");
    const prevHtml = btn.innerHTML;
    btn.disabled = true;
    btn.textContent = "Creating...";

    try {
      const res = await fetch("/api/scan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: uploadState.scanId,
          linkapi: document.getElementById("api-url").value.trim(),
          modes: selectedScanModes(),
        }),
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error);

      showToast(`Scan record ${formatScanId(data.id)} created`);
      persistApiUrlIfRemembered();
      resetUploadForm();
      collapseNewScan();
      loadScanHistory();
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      btn.innerHTML = prevHtml;
      updateCreateButton();
    }
  });
}

async function checkApiUrlConnect() {
  const input = document.getElementById("api-url");
  const btn = document.getElementById("btn-check-api-url");
  const url = input.value.trim();

  if (!url) {
    showToast("Enter API Base Path URL first", "error", 5000);
    return;
  }

  btn.disabled = true;
  const prev = btn.innerHTML;
  btn.textContent = "Checking...";

  try {
    const res = await fetch("/api/scan/check-url", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Check failed");

    persistApiUrlIfRemembered();

    if (data.connected) {
      showToast(`Connected — HTTP ${data.status} (${data.latency_ms}ms)`, "success", 5000);
    } else if (data.status != null) {
      showToast(`HTTP ${data.status} — ${data.latency_ms}ms (need <500ms)`, "error", 5000);
    } else {
      showToast(data.message || data.error || "Connection failed", "error", 5000);
    }
  } catch (err) {
    showToast(err.message, "error", 5000);
  } finally {
    btn.disabled = false;
    btn.innerHTML = prev;
  }
}

function resetUploadForm() {
  uploadState.scanId = null;
  uploadState.linksource = null;
  uploadState.linkrawswagger = null;
  document.getElementById("api-url").value = "";
  applyRememberedApiUrlToInput();
  document.getElementById("source-filename").textContent = "";
  document.getElementById("swagger-filename").textContent = "";
  document.getElementById("scan-id-label").textContent =
    "// awaiting init_scan() — generate UUID to enable uploads";
  document.querySelectorAll(".dropzone").forEach((z) => z.classList.remove("done"));
  setDropzonesEnabled(false);
}

function renderProgressCell(status, progress, phase) {
  if (!status) {
    return '<span class="muted">—</span>';
  }
  const pct = status === "done" ? 100 : Number(progress || 0);
  const label = status === "done" ? "done" : phase || status;
  const cssStatus = status === "new" ? "progress" : status;
  return `
    <div class="progress-cell">
      <div class="progress-bar" title="${label}">
        <div class="progress-fill ${cssStatus}" style="width: ${pct}%"></div>
      </div>
      <div class="progress-meta">${pct}% · ${label}</div>
    </div>`;
}

function renderHistoryRow(r) {
  const canScan = r.can_run_scan === true;
  const canViewReport = r.can_view_report === true;
  const reportsGenerating = r.reports_generating === true;
  const isQueued = r.status === "queued";
  const isCancelled = r.status === "cancel";
  const isFailed =
    r.status === "fail" || r.sast_status === "fail" || r.dast_status === "fail";
  // Keep polling while reports are still rendering so the button flips on its own.
  const isActive =
    r.status === "progress" ||
    r.status === "queued" ||
    r.sast_status === "progress" ||
    r.dast_status === "progress" ||
    reportsGenerating;

  const scanTitle = canScan
    ? "Start pipeline: SAST → DAST → Report"
    : isQueued
      ? "Waiting in queue"
      : "Scan already started or finished";

  const reportTitle = canViewReport
    ? "View / download reports"
    : reportsGenerating
      ? "Generating Executive & Technical PDFs…"
      : "Available after the reports are generated";

  // The report button is disabled until BOTH PDFs exist; a spinner shows while
  // they render. report icon vs spinner:
  const reportIcon = reportsGenerating
    ? `<span class="btn-spinner" aria-hidden="true"></span>`
    : `<svg class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`;
  const reportLabel = reportsGenerating ? "Generating…" : "Report ▾";

  return `
    <tr data-id="${r.id}" data-active="${isActive ? "1" : "0"}">
      <td title="${r.id}">${formatScanId(r.id)}</td>
      <td>
        <div class="status-cell">
          ${badge(r.status)}
          ${isFailed ? `<button type="button" class="btn-failure-details" data-id="${r.id}">View Details</button>` : ""}
        </div>
        <div class="row-modes">${formatScanModes(r.scan_modes)}</div>
      </td>
      <td title="${r.linkapi}">${truncate(r.linkapi, 36)}</td>
      <td>${renderProgressCell(r.sast_status, r.sast_progress, r.sast_phase)}${renderModelChipRow('sast', r.sast_model)}</td>
      <td>${renderProgressCell(r.dast_status, r.dast_progress, r.dast_phase)}${renderModelChipRow('dast', r.dast_model)}</td>
      <td>${new Date(r.created_at).toLocaleString()}</td>
      <td>
        <div class="actions">
          <button class="btn btn-success btn-action btn-action-scan" data-id="${r.id}" ${canScan ? "" : "disabled"} title="${scanTitle}"><svg class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/><circle cx="12" cy="12" r="2"/></svg>Scan</button>
          <div class="dropdown ${canViewReport ? "" : "dropdown-disabled"}">
            <button class="btn btn-ghost btn-action btn-action-report ${reportsGenerating ? "is-loading" : ""}" data-id="${r.id}" ${canViewReport ? "" : "disabled"} title="${reportTitle}">${reportIcon}${reportLabel}</button>
            <div class="dropdown-menu">
              <button class="dropdown-item" data-id="${r.id}" data-mode="manage" ${canViewReport ? "" : "disabled"}>View Executive</button>
              <button class="dropdown-item" data-id="${r.id}" data-mode="detail" ${canViewReport ? "" : "disabled"}>View Technical</button>
              <div class="dropdown-divider" role="separator"></div>
              <button class="dropdown-item" data-id="${r.id}" data-action="exec-pdf" ${canViewReport ? "" : "disabled"}>Executive PDF</button>
              <button class="dropdown-item" data-id="${r.id}" data-action="tech-pdf" ${canViewReport ? "" : "disabled"}>Technical PDF</button>
            </div>
          </div>
          <button class="btn btn-danger btn-action btn-action-cancel" data-id="${r.id}" ${isCancelled || r.status === "done" || r.status === "fail" ? "disabled" : ""} title="${isQueued ? "Leave queue" : "Cancel scan"}"><svg class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="7.86 2 16.14 2 22 7.86 22 16.14 16.14 22 7.86 22 2 16.14 2 7.86"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>Cancel</button>
        </div>
      </td>
    </tr>`;
}

function truncate(str, len) {
  if (!str) return "";
  return str.length > len ? str.slice(0, len) + "…" : str;
}

const POLL_INTERVAL_MS = 5000;
let scanPollTimer = null;
let pollActive = false; // true while some scan still needs updates
let historyFetchInFlight = false; // dedupe overlapping poll requests

function stopScanPolling() {
  if (scanPollTimer) {
    clearInterval(scanPollTimer);
    scanPollTimer = null;
  }
}

function scanViewActive() {
  return document.getElementById("view-scan")?.classList.contains("active");
}

function pollTick() {
  // Pause when the tab is hidden or the user is on another page.
  if (document.hidden || !scanViewActive()) return;
  loadScanHistory(true);
}

function startScanPollingIfNeeded(rows) {
  // Keep polling while any scan is progressing/queued OR its reports are still
  // generating. Terminal scans (done/fail/cancel) with reports ready don't need it.
  pollActive = rows.some(
    (r) =>
      r.status === "progress" ||
      r.status === "queued" ||
      r.sast_status === "progress" ||
      r.dast_status === "progress" ||
      r.sast_status === "new" ||
      r.dast_status === "new" ||
      r.reports_generating === true
  );
  stopScanPolling();
  if (pollActive && !document.hidden) {
    scanPollTimer = setInterval(pollTick, POLL_INTERVAL_MS);
  }
}

// Pause polling when the tab is hidden; resume + refresh immediately on return.
function setupPollingVisibility() {
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      stopScanPolling();
    } else if (pollActive && scanViewActive()) {
      loadScanHistory(true);
      if (!scanPollTimer) scanPollTimer = setInterval(pollTick, POLL_INTERVAL_MS);
    }
  });
}

async function loadScanHistory(silent = false) {
  if (silent && historyFetchInFlight) return; // skip if a poll is already running
  historyFetchInFlight = true;
  const tbody = document.getElementById("history-body");
  if (!silent) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty">Loading...</td></tr>';
  }

  try {
    const res = await fetch("/api/scans");
    const rows = await res.json();
    if (!res.ok) throw new Error(rows.error);

    if (!rows.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No scan history yet</td></tr>';
      stopScanPolling();
      pollActive = false;
      return;
    }

    tbody.innerHTML = rows.map(renderHistoryRow).join("");
    bindHistoryActions();
    startScanPollingIfNeeded(rows);
  } catch (err) {
    if (!silent) {
      tbody.innerHTML = `<tr><td colspan="7" class="empty">${err.message}</td></tr>`;
    }
  } finally {
    historyFetchInFlight = false;
  }
}

let activeReportScanId = null;
let activeReportMode = "manage"; // manage = Executive, detail = Technical

function downloadTextFile(filename, content, mime = "text/plain;charset=utf-8") {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

async function fetchReportData(id, mode) {
  const res = await fetch(`/api/scans/${id}/report?mode=${mode}`);
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load report");
  return data;
}

async function downloadReportFile(id, type, ext) {
  const res = await fetch(`/api/scans/${id}/report/file?type=${type}`);
  if (!res.ok) {
    let msg = `${ext.toUpperCase()} report not ready yet`;
    try {
      const j = await res.json();
      if (j && j.error) msg = j.error;
    } catch (_) {}
    throw new Error(msg);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `report-${formatScanId(id)}.${ext}`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

async function downloadRawReport(id) {
  await downloadReportFile(id, "html", "html");
  showToast("HTML report downloaded");
}

async function downloadExecutivePdf(id) {
  await downloadReportFile(id, "exec-pdf", "pdf");
  showToast("Executive PDF downloaded");
}

async function downloadTechnicalPdf(id) {
  await downloadReportFile(id, "tech-pdf", "pdf");
  showToast("Technical PDF downloaded");
}

function resetDropdownMenuStyles(menu) {
  if (!menu) return;
  menu.classList.remove("dropdown-menu--fixed");
  menu.style.position = "";
  menu.style.left = "";
  menu.style.top = "";
  menu.style.right = "";
  menu.style.zIndex = "";
  menu.style.minWidth = "";
}

function closeDropdownMenus() {
  document.querySelectorAll(".dropdown.open").forEach((dropdown) => {
    dropdown.classList.remove("open");
    resetDropdownMenuStyles(dropdown.querySelector(".dropdown-menu"));
  });
}

function positionDropdownMenu(dropdown) {
  const menu = dropdown.querySelector(".dropdown-menu");
  const btn = dropdown.querySelector(".btn-action-report");
  if (!menu || !btn) return;

  menu.classList.add("dropdown-menu--fixed");
  menu.style.position = "fixed";
  menu.style.right = "auto";
  menu.style.zIndex = "1200";
  menu.style.minWidth = "220px";

  const rect = btn.getBoundingClientRect();
  const menuWidth = menu.offsetWidth || 220;
  const left = Math.max(8, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8));
  let top = rect.bottom + 5;

  menu.style.left = `${left}px`;
  menu.style.top = `${top}px`;

  const menuRect = menu.getBoundingClientRect();
  if (menuRect.bottom > window.innerHeight - 8) {
    top = Math.max(8, rect.top - menuRect.height - 5);
    menu.style.top = `${top}px`;
  }
}

function bindHistoryActions() {
  document.querySelectorAll(".btn-action-scan").forEach((btn) => {
    btn.addEventListener("click", () => runScan(btn.dataset.id, btn));
  });

  document.querySelectorAll(".btn-action-cancel").forEach((btn) => {
    btn.addEventListener("click", () => cancelScan(btn.dataset.id, btn));
  });

  document.querySelectorAll(".btn-failure-details").forEach((btn) => {
    btn.addEventListener("click", () => openFailureDetails(btn.dataset.id, btn));
  });

  document.querySelectorAll(".btn-action-report").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      if (btn.disabled) return;
      e.stopPropagation();
      const dropdown = btn.closest(".dropdown");
      const wasOpen = dropdown.classList.contains("open");
      closeDropdownMenus();
      if (!wasOpen) {
        dropdown.classList.add("open");
        positionDropdownMenu(dropdown);
      }
    });
  });

  document.querySelectorAll(".dropdown-item").forEach((item) => {
    item.addEventListener("click", async (e) => {
      e.stopPropagation();
      if (item.disabled) return;
      closeDropdownMenus();
      const id = item.dataset.id;
      const mode = item.dataset.mode;
      const action = item.dataset.action;
      try {
        if (action === "exec-pdf") await downloadExecutivePdf(id);
        else if (action === "tech-pdf") await downloadTechnicalPdf(id);
        else if (mode) await openReport(id, mode);
      } catch (err) {
        showToast(err.message, "error");
      }
    });
  });
}

document.addEventListener("click", () => {
  closeDropdownMenus();
});

window.addEventListener("resize", closeDropdownMenus);
window.addEventListener("scroll", closeDropdownMenus, true);

async function runScan(id, btn) {
  const prevHtml = btn.innerHTML;
  btn.disabled = true;
  btn.textContent = "...";

  try {
    const res = await fetch(`/api/scans/${id}/run`, { method: "POST" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    if (data.status === "queued" || data.queued) {
      showToast(`Scan #${formatScanId(id)} queued — runs after the current scan finishes`);
    } else {
      showToast(`Scan #${formatScanId(id)} started — SAST → DAST → Report`);
    }
    loadScanHistory();
  } catch (err) {
    showToast(err.message, "error");
    btn.disabled = false;
    btn.innerHTML = prevHtml;
  }
}

async function cancelScan(id, btn) {
  const ok = await UI.confirmDialog.open({
    title: "Cancel Scan",
    message: "Are you sure you want to cancel this scan?",
    detail: `Scan #${formatScanId(id)}`,
    confirmText: "Cancel Scan",
    cancelText: "Keep Running",
    danger: true,
  });
  if (!ok) return;

  btn.disabled = true;

  try {
    const res = await fetch(`/api/scans/${id}/cancel`, { method: "POST" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    showToast(`Scan #${id} cancelled`);
    loadScanHistory();
  } catch (err) {
    showToast(err.message, "error");
    btn.disabled = false;
  }
}

function escHtml(s) {
  return String(s == null ? "" : s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

async function openFailureDetails(id, btn) {
  if (btn) btn.disabled = true;
  try {
    const res = await fetch(`/api/scans/${id}/failure-details`);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Failed to load failure details");
    renderFailureModal(id, data);
  } catch (err) {
    UI.toast.error(err.message || "Failed to load failure details");
  } finally {
    if (btn) btn.disabled = false;
  }
}

function renderFailureModal(id, data) {
  const ts = data.timestamp ? new Date(data.timestamp).toLocaleString() : null;
  // Build the technical block (reason/details/trace, de-duplicated).
  const techParts = [];
  for (const v of [data.details, data.trace]) {
    if (v && !techParts.includes(v)) techParts.push(v);
  }
  const tech = techParts.join("\n\n");

  // Field row helper. Hidden when value missing unless `always` (graceful).
  function field(label, value, opts = {}) {
    const has = value != null && value !== "";
    if (!has && !opts.always) return "";
    const val = has ? escHtml(value) : "—";
    const cls = ["failure-field__value", opts.mono ? "mono" : "", opts.badge ? "failure-type" : ""]
      .filter(Boolean)
      .join(" ");
    return `<div class="failure-field"><div class="failure-field__label">${label}</div><div class="${cls}">${val}</div></div>`;
  }

  const pathHtml =
    Array.isArray(data.history) && data.history.length
      ? `<div class="failure-field"><div class="failure-field__label">Execution Path</div><div class="failure-path">${data.history
          .map(
            (h) =>
              `<span class="failure-step failure-step--${escHtml(h.status)}">${escHtml(h.agent)} · ${escHtml(h.status)}</span>`
          )
          .join("")}</div></div>`
      : "";

  const body = document.createElement("div");
  body.className = "failure-details";
  body.innerHTML =
    field("Scan ID", formatScanId(id), { mono: true, always: true }) +
    field("Failed Agent", data.agent, { always: true }) +
    field("Stage", data.stage) +
    field("Error Type", data.error_type, { badge: true }) +
    field("Reason", data.reason, { always: true }) +
    field("Timestamp", ts) +
    pathHtml +
    (tech
      ? `<div class="failure-field"><div class="failure-field__label">Technical Details</div><pre class="failure-pre">${escHtml(tech)}</pre></div>`
      : "");

  const copyText = [
    `Scan ID: ${id}`,
    `Failed Agent: ${data.agent || "—"}`,
    `Stage: ${data.stage || "—"}`,
    `Error Type: ${data.error_type || "—"}`,
    `Reason: ${data.reason || "—"}`,
    ts ? `Timestamp: ${ts}` : null,
    tech ? `\nTechnical Details:\n${tech}` : null,
  ]
    .filter(Boolean)
    .join("\n");

  UI.modal.open({
    title: "Scan Failed",
    bodyNode: body,
    danger: true,
    className: "ui-modal--wide",
    actions: [
      {
        label: "Copy Details",
        variant: "btn-ghost",
        keepOpen: true,
        onClick: () => {
          if (navigator.clipboard) navigator.clipboard.writeText(copyText).catch(() => {});
          UI.toast.success("Failure details copied");
        },
      },
      { label: "Close", variant: "btn-primary", value: true, autofocus: true },
    ],
  });
}

async function openReport(id, mode) {
  try {
    const data = await fetchReportData(id, mode);
    activeReportScanId = id;
    activeReportMode = mode;

    const iframe = document.getElementById("modal-iframe");
    const body = document.getElementById("modal-body");
    iframe.classList.add("hidden");
    body.classList.remove("hidden");

    document.getElementById("modal-title").textContent =
      mode === "manage"
        ? `Executive Report (#${formatScanId(id)})`
        : `Technical Report (#${formatScanId(id)})`;

    const html = mode === "manage" ? data.highlevel_html : data.detail_html;
    if (html) {
      body.classList.add("hidden");
      iframe.classList.remove("hidden");
      iframe.srcdoc = html;
      body.textContent = "";
    } else {
      body.textContent = JSON.stringify(data, null, 2);
    }

    document.getElementById("report-modal").classList.add("open");
  } catch (err) {
    showToast(err.message, "error");
  }
}

function setupModelsPage() {
  document.getElementById("btn-check-llm").addEventListener("click", async () => {
    const btn = document.getElementById("btn-check-llm");
    btn.disabled = true;
    try {
      const res = await fetch("/api/models/check");
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      const resultEl = document.getElementById("models-llm-connect-result");
      resultEl.textContent = data.connected
        ? `Connected — ${data.message}`
        : `Failed — ${data.message}`;
      showToast(
        data.connected ? `LLM OK — ${data.message}` : `LLM error — ${data.message}`,
        data.connected ? "success" : "error"
      );
      loadModelsPage();
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      btn.disabled = false;
    }
  });

  document.querySelectorAll(".btn-save-agent-model").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const agent = btn.dataset.agent;
      const select = document.getElementById(`assign-${agent}`);
      const modelName = select.value;
      if (!modelName) {
        showToast(`Select a model for ${agent.toUpperCase()}`, "error");
        return;
      }

      btn.disabled = true;
      try {
        const res = await fetch(`/api/models/assignments/${agent}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ model_name: modelName }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        showToast(`${agent.toUpperCase()} model saved: ${modelName}`);
        loadModelsPage();
      } catch (err) {
        showToast(err.message || "Failed to save model", "error");
      } finally {
        btn.disabled = false;
      }
    });
  });
}

function renderModelsTable(usableSet) {
  const tbody = document.getElementById("models-list");
  const { modelNames, remoteById, assignmentMap } = modelsPageState;

  if (!modelNames.length) {
    tbody.innerHTML = '<tr><td colspan="4" class="empty">No models from LLM API — check connection first</td></tr>';
    return;
  }

  tbody.innerHTML = modelNames
    .map((name) => {
      const assigned = [];
      if (assignmentMap.sast === name) assigned.push("SAST");
      if (assignmentMap.dast === name) assigned.push("DAST");
      if (assignmentMap.report === name) assigned.push("Report");
      const provider = remoteById[name]?.owned_by || (remoteById[name] ? "remote" : "env");

      let availCell;
      if (usableSet === null) {
        availCell = '<span class="muted">Checking…</span>';
      } else if (usableSet.has(name)) {
        availCell = '<span class="avail-badge avail-yes">✓ Available</span>';
      } else {
        availCell = '<span class="avail-badge avail-no">✗ Unavailable</span>';
      }

      return `<tr>
        <td>${name}</td>
        <td>${provider}</td>
        <td>${assigned.length ? assigned.join(", ") : "—"}</td>
        <td>${availCell}</td>
      </tr>`;
    })
    .join("");
}

function updateConnectionSummary(usableSet) {
  const { connected, modelNames } = modelsPageState;
  if (!connected) return;
  const connectEl = document.getElementById("models-llm-connect-result");
  if (!connectEl) return;
  const totalModels = modelNames.length;
  const availableCount = usableSet ? usableSet.size : 0;
  connectEl.textContent = `Connected · ${totalModels} models / ${availableCount} available for use`;
}

async function loadModelsPage() {
  const tbody = document.getElementById("models-list");
  tbody.innerHTML = '<tr><td colspan="4" class="empty">Loading...</td></tr>';

  try {
    const res = await fetch("/api/models");
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    const statusEl = document.getElementById("models-llm-status");
    const connected = data.llm_connected === true;
    statusEl.textContent = connected ? "● Connected" : "● Offline";
    statusEl.className = `llm-status ${connected ? "up" : "down"}`;

    document.getElementById("models-llm-base").textContent =
      `API base: ${data.llm_health?.api_base || data.llm_health?.message || "—"}`;

    const connectEl = document.getElementById("models-llm-connect-result");
    if (data.llm_connected) {
      connectEl.textContent = data.llm_health?.message || "Connected";
    } else if (connectEl.textContent === "—") {
      connectEl.textContent = "Not checked yet — click Check LLM Connection";
    }

    const assignmentMap = Object.fromEntries(
      (data.assignments || []).map((a) => [a.agent_type, a.model_name])
    );
    document.getElementById("models-agent-config-summary").textContent =
      `Current — SAST: ${assignmentMap.sast || "—"}, DAST: ${assignmentMap.dast || "—"}, Report: ${assignmentMap.report || "—"}`;

    const remoteById = Object.fromEntries((data.remote_models || []).map((m) => [m.id, m]));
    const modelNames = data.models || [];
    const sastModelNames = data.sast_models?.length ? data.sast_models : modelNames;

    modelsPageState = { modelNames, remoteById, assignmentMap, connected };

    fillAssignmentSelect("assign-sast", sastModelNames, data.assignments, "sast");
    fillAssignmentSelect("assign-dast", modelNames, data.assignments, "dast");
    fillAssignmentSelect("assign-report", modelNames, data.assignments, "report");
    ["assign-sast", "assign-dast", "assign-report"].forEach((id) =>
      UI.Dropdown.enhance(document.getElementById(id))
    );

    // Render table immediately with "Checking…" availability; validation will update it.
    renderModelsTable(null);

    // Asynchronously validate which models are actually usable; updates dropdowns,
    // table availability column, and connection summary when done.
    validateModelDropdowns();
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty">${err.message}</td></tr>`;
  }
}

function fillAssignmentSelect(selectId, modelNames, assignments, agentType) {
  const select = document.getElementById(selectId);
  const current = assignments.find((a) => a.agent_type === agentType)?.model_name || "";
  const options = modelNames.length ? modelNames : [current].filter(Boolean);

  if (!options.length) {
    select.innerHTML = '<option value="">No models available</option>';
    return;
  }

  select.innerHTML = options
    .map((name) => `<option value="${name}" ${name === current ? "selected" : ""}>${name}</option>`)
    .join("");
}

let modelValidationInFlight = false;

// Validate models against the LLM API and narrow each agent dropdown to only the
// usable ones. Also updates the availability column in the models table and the
// connection summary. Shows a loading state on the dropdowns while validating.
async function validateModelDropdowns() {
  if (modelValidationInFlight) return;
  modelValidationInFlight = true;
  const ids = ["assign-sast", "assign-dast", "assign-report"];
  ids.forEach((id) => document.getElementById(id)?.__uiDropdown?.setLoading(true));
  try {
    const res = await fetch("/api/models/usable");
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Model validation failed");
    const usable = Array.isArray(data.usable) ? data.usable : [];
    const usableSet = new Set(usable);

    ids.forEach((id) => {
      const select = document.getElementById(id);
      if (!select) return;
      const current = select.value;
      const opts = usable.slice();
      // Preserve the current assignment even if it didn't re-validate this round.
      if (current && !opts.includes(current)) opts.unshift(current);

      if (!opts.length) {
        select.innerHTML = '<option value="">No usable models available</option>';
        select.disabled = true;
      } else {
        select.disabled = false;
        select.innerHTML = opts
          .map((n) => `<option value="${n}" ${n === current ? "selected" : ""}>${n}</option>`)
          .join("");
      }
      const ctrl = UI.Dropdown.enhance(select);
      ctrl.setLoading(false);
    });

    // Update the models table availability column and connection summary
    renderModelsTable(usableSet);
    updateConnectionSummary(usableSet);
  } catch (err) {
    // Graceful fallback: keep the unvalidated list usable, just clear loading.
    ids.forEach((id) => document.getElementById(id)?.__uiDropdown?.setLoading(false));
    showToast(`Model validation failed: ${err.message || "provider error"}`, "error");
  } finally {
    modelValidationInFlight = false;
  }
}

async function rerunReport(scanId) {
  const res = await fetch(`/api/scans/${scanId}/report/rerun`, { method: "POST" });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to regenerate report");
  return data;
}

function setupModal() {
  document.getElementById("modal-close").addEventListener("click", () => {
    document.getElementById("report-modal").classList.remove("open");
    activeReportScanId = null;
  });

  document.getElementById("modal-download-raw").addEventListener("click", async () => {
    if (!activeReportScanId) return;
    try {
      const type = activeReportMode === "detail" ? "tech-html" : "exec-html";
      await downloadReportFile(activeReportScanId, type, "html");
      showToast("HTML report downloaded");
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  document.getElementById("modal-download-pdf").addEventListener("click", async () => {
    if (!activeReportScanId) return;
    try {
      if (activeReportMode === "detail") await downloadTechnicalPdf(activeReportScanId);
      else await downloadExecutivePdf(activeReportScanId);
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  document.getElementById("modal-rerun-report").addEventListener("click", async () => {
    if (!activeReportScanId) return;
    const btn = document.getElementById("modal-rerun-report");
    const prev = btn.textContent;
    btn.disabled = true;
    btn.textContent = "Regenerating…";
    try {
      await rerunReport(activeReportScanId);
      showToast("Report regeneration started — refresh in a minute", "success", 6000);
      loadScanHistory(true);
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      btn.disabled = false;
      btn.textContent = prev;
    }
  });

  document.getElementById("report-modal").addEventListener("click", (e) => {
    if (e.target.id === "report-modal") {
      document.getElementById("report-modal").classList.remove("open");
      activeReportScanId = null;
    }
  });
}

const skillsState = {
  agent: "sast",
  currentPath: null,
  isNew: false,
  dirty: false,
};

function setSkillsEditorEnabled(enabled) {
  if (window.SkillsEditor) {
    SkillsEditor.setReadOnly(!enabled);
  }
  document.getElementById("btn-skill-save").disabled = !enabled;
  document.getElementById("btn-skill-delete").disabled = !enabled || skillsState.isNew;
}

function markSkillsDirty(dirty) {
  skillsState.dirty = dirty;
  const pathEl = document.getElementById("skills-current-path");
  if (!skillsState.currentPath) return;
  const suffix = dirty ? " *" : "";
  pathEl.textContent = `${skillsState.currentPath}${suffix}`;
}

async function loadSkillsFileList() {
  const listEl = document.getElementById("skills-file-list");
  listEl.innerHTML = '<li class="empty">Loading...</li>';

  try {
    const res = await fetch(`/api/skills/${skillsState.agent}`);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    document.getElementById("skills-prefix-label").textContent = `MinIO: ${data.prefix}`;

    if (!data.files?.length) {
      listEl.innerHTML = '<li class="empty">No skill files yet — create one</li>';
      return;
    }

    listEl.innerHTML = data.files
      .map(
        (f) => `
        <li>
          <button type="button" class="skills-file-item ${f.path === skillsState.currentPath ? "active" : ""}"
            data-path="${f.path}">
            ${f.path}
          </button>
        </li>`
      )
      .join("");

    listEl.querySelectorAll(".skills-file-item").forEach((btn) => {
      btn.addEventListener("click", () => openSkillFile(btn.dataset.path));
    });
  } catch (err) {
    listEl.innerHTML = `<li class="empty">${err.message}</li>`;
  }
}

// Shared "discard unsaved changes?" confirmation used across the skills editor.
function confirmDiscardChanges() {
  return UI.confirmDialog.open({
    title: "Discard changes?",
    message: "You have unsaved changes that will be lost.",
    confirmText: "Discard",
    cancelText: "Keep Editing",
    danger: true,
  });
}

async function openSkillFile(path) {
  if (skillsState.dirty && !(await confirmDiscardChanges())) return;

  try {
    const res = await fetch(
      `/api/skills/${skillsState.agent}/content?path=${encodeURIComponent(path)}`
    );
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    skillsState.currentPath = path;
    skillsState.isNew = false;
    skillsState.dirty = false;

    if (window.SkillsEditor) SkillsEditor.setValue(data.content || "");
    document.getElementById("skills-current-path").textContent = path;
    setSkillsEditorEnabled(true);
    if (window.SkillsEditor) SkillsEditor.focus();
    await loadSkillsFileList();
  } catch (err) {
    showToast(err.message, "error");
  }
}

function clearSkillEditor() {
  skillsState.currentPath = null;
  skillsState.isNew = false;
  skillsState.dirty = false;
  if (window.SkillsEditor) SkillsEditor.setValue("");
  document.getElementById("skills-current-path").textContent = "Select a file";
  setSkillsEditorEnabled(false);
}

async function loadSkillsPage() {
  await loadSkillsFileList();
  if (!skillsState.currentPath) clearSkillEditor();
}

async function saveSkillFile() {
  const path = skillsState.currentPath;
  const content = window.SkillsEditor ? SkillsEditor.getValue() : "";
  if (!path) return;

  const method = skillsState.isNew ? "POST" : "PUT";
  const btn = document.getElementById("btn-skill-save");
  btn.disabled = true;

  try {
    const res = await fetch(
      `/api/skills/${skillsState.agent}/content?path=${encodeURIComponent(path)}`,
      {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      }
    );
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    skillsState.isNew = false;
    skillsState.dirty = false;
    markSkillsDirty(false);
    document.getElementById("btn-skill-delete").disabled = false;
    showToast(`Saved ${path}`);
    await loadSkillsFileList();
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    btn.disabled = false;
  }
}

async function deleteSkillFile() {
  const path = skillsState.currentPath;
  if (!path || skillsState.isNew) return;
  const ok = await UI.confirmDialog.open({
    title: "Delete Skill File",
    message: "Are you sure you want to delete this skill file? This cannot be undone.",
    detail: path,
    confirmText: "Delete",
    cancelText: "Cancel",
    danger: true,
  });
  if (!ok) return;

  try {
    const res = await fetch(
      `/api/skills/${skillsState.agent}/content?path=${encodeURIComponent(path)}`,
      { method: "DELETE" }
    );
    const data = await res.json();
    if (!res.ok) throw new Error(data.error);

    showToast(`Deleted ${path}`);
    clearSkillEditor();
    await loadSkillsFileList();
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function promptNewSkillFile() {
  if (skillsState.dirty && !(await confirmDiscardChanges())) return;

  const path = await UI.confirmDialog.prompt({
    title: "New Skill File",
    message: "Enter the new skill path:",
    placeholder: "references/sqli.md",
    confirmText: "Create",
  });
  if (!path) return;

  const trimmed = path.trim().replace(/^\/+/, "");
  if (!trimmed) {
    showToast("Invalid path", "error");
    return;
  }

  skillsState.currentPath = trimmed;
  skillsState.isNew = true;
  skillsState.dirty = true;
  if (window.SkillsEditor) SkillsEditor.setValue("");
  document.getElementById("skills-current-path").textContent = `${trimmed} *`;
  setSkillsEditorEnabled(true);
  document.getElementById("btn-skill-delete").disabled = true;
  if (window.SkillsEditor) SkillsEditor.focus();
}

function setupSkillsPage() {
  if (window.SkillsEditor) {
    SkillsEditor.init();
    SkillsEditor.onChange(() => {
      if (!skillsState.currentPath) return;
      markSkillsDirty(true);
    });
  }
  document.querySelectorAll(".skills-agent-tab").forEach((btn) => {
    btn.addEventListener("click", async () => {
      if (skillsState.dirty && !(await confirmDiscardChanges())) return;

      document.querySelectorAll(".skills-agent-tab").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      skillsState.agent = btn.dataset.agent;
      clearSkillEditor();
      loadSkillsFileList();
    });
  });

  document.getElementById("btn-skill-refresh").addEventListener("click", () => {
    loadSkillsPage();
  });

  document.getElementById("btn-skill-new").addEventListener("click", promptNewSkillFile);

  document.getElementById("btn-skill-save").addEventListener("click", saveSkillFile);

  document.getElementById("btn-skill-delete").addEventListener("click", deleteSkillFile);
}

const THEME_STORAGE_KEY = "agentsdast.theme";

function applyTheme(theme) {
  const t = theme === "dark" ? "dark" : "light";
  document.documentElement.setAttribute("data-theme", t);
  const label = document.querySelector("#theme-toggle .theme-toggle-label");
  // Label/icon advertise the action: in light mode it switches to dark, and vice-versa.
  if (label) label.textContent = t === "light" ? "Dark mode" : "Light mode";
  if (window.SkillsEditor && typeof SkillsEditor.refresh === "function") {
    SkillsEditor.refresh();
  }
}

function setupTheme() {
  let saved = "light";
  try {
    saved = localStorage.getItem(THEME_STORAGE_KEY) || "light";
  } catch (_) {}
  applyTheme(saved);

  const toggle = document.getElementById("theme-toggle");
  if (!toggle) return;
  toggle.addEventListener("click", () => {
    const current = document.documentElement.getAttribute("data-theme") || "light";
    const next = current === "light" ? "dark" : "light";
    applyTheme(next);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, next);
    } catch (_) {}
  });
}

document.addEventListener("DOMContentLoaded", () => {
  setupTheme();
  initRouter();
  restoreRememberedApiUrl();
  setupDropzones();
  setupModelsPage();
  setupSkillsPage();
  setupModal();
  setupPollingVisibility();
});
