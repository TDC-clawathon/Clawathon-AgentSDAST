const express = require("express");
const { v4: uuidv4 } = require("uuid");
const { getPool } = require("../db");
const orchestrator = require("../agents/orchestrator");
const agents = require("../agents/client");
const { newScanId } = require("../utils/ids");
const { loadScanReports, getObjectBuffer, reportPdfsReady } = require("../storage/reportArtifacts");
const {
  getScanRecord,
  hasAgentRecords,
} = require("../services/scan");

const router = express.Router();

const SAST_MODES = ["quickscan", "deepscan", "pgwscan"];

// normalizeScanModes stores the SELECTED mode: Quick and Deep are mutually
// exclusive (Deep supersedes Quick and already includes its skills); PGW is an
// independent add-on. The AgentSAST engine expands deepscan -> quickscan+deepscan
// at load time, so we only persist the user's choice here.
function normalizeScanModes(input) {
  const want = new Set(
    (Array.isArray(input) ? input : String(input || "").split(","))
      .map((m) => String(m).trim().toLowerCase())
      .filter(Boolean)
  );
  const out = [want.has("deepscan") ? "deepscan" : "quickscan"];
  if (want.has("pgwscan")) out.push("pgwscan");
  return out;
}

const SCAN_LIST_QUERY = `
  SELECT
    sr.id,
    sr.linksource,
    sr.linkrawswagger,
    sr.linkapi,
    sr.sastid,
    sr.dastid,
    sr.status,
    sr.scan_modes,
    sr.sast_model,
    sr.dast_model,
    sr.created_at,
    s.status AS sast_status,
    s.progress AS sast_progress,
    COALESCE(s.phase, s.last_message) AS sast_phase,
    d.status AS dast_status,
    d.progress AS dast_progress,
    COALESCE(d.phase, d.error_msg) AS dast_phase
  FROM ScanRecord sr
  LEFT JOIN sast s ON s.id = (
    SELECT s2.id FROM sast s2
    WHERE s2.project_id = sr.id
    ORDER BY
      CASE LOWER(s2.status)
        WHEN 'done' THEN 0
        WHEN 'fail' THEN 1
        WHEN 'failed' THEN 1
        WHEN 'cancel' THEN 2
        WHEN 'cancelled' THEN 2
        WHEN 'canceled' THEN 2
        WHEN 'progress' THEN 3
        WHEN 'processing' THEN 3
        WHEN 'process' THEN 3
        WHEN 'running' THEN 3
        ELSE 4
      END,
      s2.last_update DESC
    LIMIT 1
  )
  LEFT JOIN dast d ON d.id = (
    SELECT d2.id FROM dast d2
    WHERE d2.project_id = sr.id
    ORDER BY
      CASE LOWER(d2.status)
        WHEN 'done' THEN 0
        WHEN 'fail' THEN 1
        WHEN 'failed' THEN 1
        WHEN 'cancel' THEN 2
        WHEN 'cancelled' THEN 2
        WHEN 'canceled' THEN 2
        WHEN 'progress' THEN 3
        WHEN 'processing' THEN 3
        WHEN 'process' THEN 3
        WHEN 'running' THEN 3
        ELSE 4
      END,
      d2.last_update DESC
    LIMIT 1
  )
  WHERE sr.status != 'draft'
  ORDER BY sr.created_at DESC
`;

function mapUiStatus(status) {
  if (!status) return null;
  return agents.normalizeStatus(status);
}

function mapUiProgress(status, progress) {
  const normalized = mapUiStatus(status);
  if (normalized === "done") return 100;
  if (normalized === "fail" || normalized === "cancel") return 0;
  const pct = Number(progress || 0);
  if (pct > 0) return pct;
  if (normalized === "progress") return 50;
  return 0;
}

async function enrichScanRows(rows) {
  return Promise.all(
    rows.map(async (row) => {
      let sastStatus = mapUiStatus(row.sast_status);
      let dastStatus = mapUiStatus(row.dast_status);
      let sastPhase = row.sast_phase || null;
      let dastPhase = row.dast_phase || null;

      if (!sastStatus && row.sastid) {
        sastStatus = "new";
        sastPhase = sastPhase || "Starting SAST";
      }
      if (!dastStatus && row.dastid) {
        dastStatus = "new";
        dastPhase = dastPhase || "Queued";
      }

      const sastProgress = mapUiProgress(sastStatus, row.sast_progress);
      const dastProgress = mapUiProgress(dastStatus, row.dast_progress);

      // The Report button is enabled only when BOTH report PDFs exist. While DAST
      // is done but the PDFs are still rendering, reports_generating drives the
      // spinner. Only probe MinIO once DAST is done (keeps the list cheap).
      let reportsReady = false;
      if (dastStatus === "done") {
        reportsReady = (await reportPdfsReady(row.id)).ready;
      }

      return {
        ...row,
        sast_status: sastStatus,
        dast_status: dastStatus,
        sast_phase: sastPhase,
        dast_phase: dastPhase,
        sast_progress: sastProgress,
        dast_progress: dastProgress,
        overall_progress: Math.round((sastProgress + dastProgress) / 2),
        can_run_scan: row.status === "new",
        reports_ready: reportsReady,
        reports_generating: dastStatus === "done" && !reportsReady,
        can_view_report: reportsReady,
      };
    })
  );
}

router.get("/", async (_req, res) => {
  try {
    await orchestrator.syncAllRunningJobs();

    const pool = getPool();
    const [rows] = await pool.execute(SCAN_LIST_QUERY);
    res.json(await enrichScanRows(rows));
  } catch (err) {
    console.error("Failed to list scans:", err);
    res.status(500).json({ error: "Failed to list scan records" });
  }
});

async function initScan(_req, res) {
  try {
    const scanId = newScanId();
    const pool = getPool();
    await pool.execute(
      `INSERT INTO ScanRecord (id, linksource, linkrawswagger, linkapi, status)
       VALUES (?, NULL, NULL, NULL, 'draft')`,
      [scanId]
    );

    res.status(201).json({ id: scanId, status: "draft" });
  } catch (err) {
    console.error("Scan init failed:", err);
    res.status(500).json({ error: "Failed to initialize scan" });
  }
}

async function finalizeScan(req, res) {
  try {
    const { id, linkapi, modes } = req.body;

    if (!id || !linkapi) {
      return res.status(400).json({ error: "id and linkapi are required" });
    }

    const record = await getScanRecord(id);
    if (!record) {
      return res.status(404).json({ error: "Scan record not found" });
    }
    if (record.status !== "draft") {
      return res.status(400).json({ error: "Scan is not in draft status" });
    }
    if (!record.linksource || !record.linkrawswagger) {
      return res.status(400).json({ error: "Source and swagger files must be uploaded first" });
    }

    const scanModes = normalizeScanModes(modes);
    const pool = getPool();
    await pool.execute(
      `UPDATE ScanRecord SET linkapi = ?, scan_modes = ?, status = 'new' WHERE id = ?`,
      [linkapi, scanModes.join(","), id]
    );

    res.status(201).json({
      id,
      linksource: record.linksource,
      linkrawswagger: record.linkrawswagger,
      linkapi,
      scan_modes: scanModes,
      sastid: null,
      dastid: null,
      status: "new",
      can_run_scan: true,
    });
  } catch (err) {
    console.error("Scan record creation failed:", err);
    res.status(500).json({ error: "Failed to create scan record" });
  }
}

router.get("/:id/status", async (req, res) => {
  try {
    const snapshot = await orchestrator.refreshScanRecordStatus(req.params.id);
    if (!snapshot) {
      return res.status(404).json({ error: "Scan record not found" });
    }

    const { record, sast, dast } = snapshot;
    const overallProgress = Math.round(
      ((Number(sast?.progress) || 0) + (Number(dast?.progress) || 0)) / 2
    );

    const dastDone = dast && agents.normalizeStatus(dast.status) === "done";
    const pdfs = dastDone
      ? await reportPdfsReady(record.id)
      : { executive_pdf: false, technical_pdf: false, ready: false };

    res.json({
      scan_id: record.id,
      status: record.status,
      scan_modes: normalizeScanModes(record.scan_modes),
      overall_progress: overallProgress,
      reports_ready: pdfs.ready,
      reports_generating: Boolean(dastDone) && !pdfs.ready,
      executive_pdf: pdfs.executive_pdf,
      technical_pdf: pdfs.technical_pdf,
      sast: sast
        ? {
            job_id: sast.id,
            status: sast.status,
            progress: sast.progress,
            phase: sast.phase,
            error_msg: sast.error_msg,
          }
        : null,
      dast: dast
        ? {
            job_id: dast.id,
            status: dast.status,
            progress: dast.progress,
            phase: dast.phase,
            error_msg: dast.error_msg,
          }
        : null,
    });
  } catch (err) {
    console.error("Failed to get scan status:", err);
    res.status(500).json({ error: "Failed to get scan status" });
  }
});

// Human-facing agent names per pipeline stage.
const AGENT_LABELS = { sast: "SAST Analyzer", dast: "DAST Auditor", report: "Report Generator" };

// classifyError derives a coarse error category from the failure message.
function classifyError(msg) {
  const m = String(msg || "").toLowerCase();
  if (/\btimed?\s*out\b|timeout|deadline/.test(m)) return "TimeoutError";
  if (/rate.?limit|\b429\b|too many requests/.test(m)) return "RateLimitError";
  if (/unauthor|forbidden|\b401\b|\b403\b|api[\s_-]?key|invalid key/.test(m)) return "AuthError";
  if (/econnrefused|unreachable|connection|dial|network|refused|\bEOF\b/i.test(m)) return "ConnectionError";
  if (/openapi|swagger|yaml|invalid|validation|parse/.test(m)) return "ValidationError";
  if (/no model|model .*not|configure|missing both|not configured/.test(m)) return "ConfigError";
  if (!m) return "UnknownError";
  return "ExecutionError";
}

// GET /api/scans/:id/failure-details — failure info for a failed scan.
router.get("/:id/failure-details", async (req, res) => {
  const scanId = req.params.id;
  try {
    const record = await orchestrator.syncScanJobIds(scanId);
    if (!record) return res.status(404).json({ error: "Scan record not found" });

    const stages = [];
    for (const key of ["sast", "dast", "report"]) {
      const row = await orchestrator.getLatestJobRow(key, scanId);
      if (row) stages.push({ key, agent: AGENT_LABELS[key], row });
    }

    // Execution path leading to the failure (each stage's outcome).
    const history = stages.map((s) => ({
      agent: s.agent,
      status: agents.normalizeStatus(s.row.status),
      stage: s.row.phase || s.row.last_message || null,
    }));

    // The failed stage: earliest in pipeline order with a fail status.
    const failed = stages.find(
      (s) => agents.normalizeStatus(s.row.status) === agents.SCAN_STATUS.FAIL
    );

    if (!failed) {
      return res.json({ failed: false, scan_id: scanId, status: record.status, history });
    }

    const row = failed.row;
    const reason =
      row.error_msg || row.last_message || "Execution failed (no message reported).";

    return res.json({
      failed: true,
      scan_id: scanId,
      agent: failed.agent,
      stage: row.phase || row.last_message || null,
      reason,
      timestamp: row.last_update || null,
      error_type: classifyError(reason),
      details: reason,
      trace: null, // no separate stack trace is captured; reason holds the detail
      history,
    });
  } catch (err) {
    console.error("Failed to load failure details:", err);
    res.status(500).json({ error: "Failed to load failure details" });
  }
});

router.post("/:id/run", async (req, res) => {
  const scanId = req.params.id;

  try {
    const record = await getScanRecord(scanId);
    if (!record) {
      return res.status(404).json({ error: "Scan record not found" });
    }

    if (![agents.SCAN_STATUS.NEW, agents.SCAN_STATUS.QUEUED].includes(agents.normalizeStatus(record.status))) {
      return res.status(409).json({ error: `Scan cannot start in status ${record.status}` });
    }

    const result = await orchestrator.startScanPipeline(scanId);
    if (!result.ok) {
      return res.status(409).json({ error: result.error });
    }

    res.json(result);
  } catch (err) {
    console.error("Failed to run scan:", err);
    res.status(500).json({ error: err.message || "Failed to start scan" });
  }
});

router.post("/:id/cancel", async (req, res) => {
  const scanId = req.params.id;

  try {
    const record = await orchestrator.syncScanJobIds(scanId);
    if (!record) {
      return res.status(404).json({ error: "Scan record not found" });
    }

    if (agents.normalizeStatus(record.status) === agents.SCAN_STATUS.QUEUED) {
      const pool = getPool();
      await pool.execute(`UPDATE ScanRecord SET status = 'cancel' WHERE id = ?`, [scanId]);
      await orchestrator.tryDequeueNextScan();
      return res.json({ id: scanId, status: "cancel", message: "Removed from queue" });
    }

    const results = {};
    if (record.sastid) {
      results.sast = await orchestrator.cancelAgentJob("sast", record.sastid);
    }
    if (record.dastid) {
      results.dast = await orchestrator.cancelAgentJob("dast", record.dastid);
    }
    if (record.reportid) {
      results.report = await orchestrator.cancelAgentJob("report", record.reportid);
    }

    const pool = getPool();
    await pool.execute(
      `UPDATE ScanRecord SET status = 'cancel' WHERE id = ?`,
      [scanId]
    );

    await orchestrator.tryDequeueNextScan();

    res.json({ id: scanId, status: "cancel", agent_results: results });
  } catch (err) {
    console.error("Failed to cancel scan:", err);
    res.status(500).json({ error: "Failed to cancel scan" });
  }
});

router.get("/:id/report", async (req, res) => {
  const scanId = req.params.id;
  const mode = req.query.mode === "detail" ? "detail" : "manage";

  try {
    const record = await orchestrator.syncScanJobIds(scanId);
    if (!record) {
      return res.status(404).json({ error: "Scan record not found" });
    }

    const sast = await orchestrator.getLatestJobRow("sast", scanId);
    const dast = await orchestrator.getLatestJobRow("dast", scanId);
    const report = await orchestrator.getLatestJobRow("report", scanId);

    if (!dast || agents.normalizeStatus(dast.status) !== agents.SCAN_STATUS.DONE) {
      return res.status(409).json({
        error: "Report is available after DAST completes",
      });
    }

    const artifacts = await loadScanReports(scanId);

    if (mode === "manage") {
      return res.json({
        mode: "manage",
        scan_id: scanId,
        status: record.status,
        sast: sast ? { id: sast.id, status: sast.status, result_path: sast.result_path } : null,
        dast: dast ? { id: dast.id, status: dast.status, result_path: dast.result_path } : null,
        report: report
          ? { id: report.id, status: report.status, result_path: report.result_path }
          : null,
        highlevel_html: artifacts.highlevel_html,
        stats: artifacts.stats,
      });
    }

    res.json({
      mode: "detail",
      scan: record,
      sast,
      dast,
      report,
      detail_html: artifacts.detail_html,
      highlevel_html: artifacts.highlevel_html,
      stats: artifacts.stats,
    });
  } catch (err) {
    console.error("Failed to load report:", err);
    res.status(500).json({ error: "Failed to load report" });
  }
});

router.post("/:id/report/rerun", async (req, res) => {
  try {
    const result = await orchestrator.rerunReportJob(req.params.id);
    if (!result.ok) {
      return res.status(400).json({ error: result.error });
    }
    res.json(result);
  } catch (err) {
    console.error("Failed to rerun report:", err);
    res.status(500).json({ error: "Failed to rerun report" });
  }
});

// Report artifact downloads. type selects which file:
//   exec-pdf  -> highlevel.pdf (Executive PDF)
//   tech-pdf  -> detail.pdf    (Technical PDF)
//   exec-html -> highlevel.html
//   tech-html / html -> detail.html (raw)
//   pdf (legacy) -> highlevel.pdf
const REPORT_FILES = {
  "exec-pdf": { name: "highlevel.pdf", ext: "pdf", ct: "application/pdf", label: "executive" },
  "tech-pdf": { name: "detail.pdf", ext: "pdf", ct: "application/pdf", label: "technical" },
  "exec-html": { name: "highlevel.html", ext: "html", ct: "text/html; charset=utf-8", label: "executive" },
  "tech-html": { name: "detail.html", ext: "html", ct: "text/html; charset=utf-8", label: "technical" },
  pdf: { name: "highlevel.pdf", ext: "pdf", ct: "application/pdf", label: "executive" },
  html: { name: "detail.html", ext: "html", ct: "text/html; charset=utf-8", label: "technical" },
};

router.get("/:id/report/file", async (req, res) => {
  const scanId = req.params.id;
  const spec = REPORT_FILES[req.query.type] || REPORT_FILES.html;
  const prefix = `${scanId}/report`;

  try {
    const dast = await orchestrator.getLatestJobRow("dast", scanId);
    if (!dast || agents.normalizeStatus(dast.status) !== agents.SCAN_STATUS.DONE) {
      return res.status(409).json({ error: "Report files are available after DAST completes" });
    }

    const obj = await getObjectBuffer(`${prefix}/${spec.name}`);
    if (!obj) {
      return res
        .status(404)
        .json({ error: `${spec.label} ${spec.ext.toUpperCase()} report not ready yet` });
    }

    const safeId = String(scanId).replace(/[^a-zA-Z0-9._-]/g, "_");
    const filename = `${spec.label}-report-${safeId}.${spec.ext}`;
    res.setHeader("Content-Type", spec.ct);
    res.setHeader("Content-Disposition", `attachment; filename="${filename}"`);
    res.send(obj.buffer);
  } catch (err) {
    console.error("Failed to download report file:", err);
    res.status(500).json({ error: "Failed to download report file" });
  }
});

const { assertSafeHttpUrl, safeFetch } = require("../security/ssrf");

async function checkApiUrl(req, res) {
  const url = String(req.body?.url || "").trim();
  const timeoutMs = 5000;
  const fastMs = 500;

  if (!url) {
    return res.status(400).json({ error: "url is required" });
  }

  try {
    await assertSafeHttpUrl(url);
  } catch (err) {
    return res.status(400).json({ error: err.message || "URL is not allowed" });
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  const started = Date.now();

  try {
    const response = await safeFetch(url, {
      method: "GET",
      signal: controller.signal,
      headers: { Accept: "*/*", "User-Agent": "AgentSDAST-Manager/1.0" },
    });

    const latencyMs = Date.now() - started;
    const connected = latencyMs < fastMs;

    res.json({
      ok: connected,
      connected,
      url,
      status: response.status,
      latency_ms: latencyMs,
      message: connected
        ? `Connected — HTTP ${response.status} in ${latencyMs}ms`
        : `HTTP ${response.status} in ${latencyMs}ms (slower than ${fastMs}ms)`,
    });
  } catch (err) {
    const latencyMs = Date.now() - started;
    const timedOut = err.name === "AbortError";
    res.json({
      ok: false,
      connected: false,
      url,
      status: null,
      latency_ms: latencyMs,
      error: timedOut ? `Request timed out after ${timeoutMs}ms` : err.message,
      message: timedOut ? `Timeout after ${timeoutMs}ms` : err.message,
    });
  } finally {
    clearTimeout(timer);
  }
}

module.exports = router;
module.exports.initScan = initScan;
module.exports.finalizeScan = finalizeScan;
module.exports.checkApiUrl = checkApiUrl;
