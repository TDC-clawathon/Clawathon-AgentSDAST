/**
 * Agent HTTP client — unified AgentSAST & AgentDAST API.
 *
 * Both agents expose (under /api/sast or /api/dast):
 *   GET  /health
 *   POST /run        { id?, project_id, base_url }
 *   GET  /status?id=
 */

const { resolveObjectKey } = require("../storage/minio");

const TIMEOUT_MS = Number(process.env.AGENT_REQUEST_TIMEOUT_MS || 30000);

const SCAN_STATUS = Object.freeze({
  NEW: "new",
  QUEUED: "queued",
  PROGRESS: "progress",
  DONE: "done",
  CANCEL: "cancel",
  FAIL: "fail",
});

const TERMINAL_STATUSES = new Set([
  SCAN_STATUS.DONE,
  SCAN_STATUS.CANCEL,
  SCAN_STATUS.FAIL,
]);

const AGENTS = {
  sast: {
    type: "sast",
    label: "AgentSAST",
    baseUrl: process.env.AGENT_SAST_URL || "http://localhost:8001/api/sast",
  },
  dast: {
    type: "dast",
    label: "AgentDAST",
    baseUrl: process.env.AGENT_DAST_URL || "http://localhost:8002/api/dast",
  },
  report: {
    type: "report",
    label: "AgentReport",
    baseUrl: process.env.AGENT_REPORT_URL || "http://localhost:8003/api/report",
  },
};

const ROUTES = {
  health: "/health",
  run: "/run",
  status: (id) => `/status?id=${encodeURIComponent(id)}`,
};

const LEGACY_STATUS = {
  pending: SCAN_STATUS.NEW,
  running: SCAN_STATUS.PROGRESS,
  completed: SCAN_STATUS.DONE,
  cancelled: SCAN_STATUS.CANCEL,
  canceled: SCAN_STATUS.CANCEL,
  failed: SCAN_STATUS.FAIL,
  process: SCAN_STATUS.PROGRESS,
  processing: SCAN_STATUS.PROGRESS,
};

function clampProgress(value) {
  const n = Number(value);
  if (Number.isNaN(n)) return 0;
  return Math.min(100, Math.max(0, Math.round(n)));
}

function normalizeStatus(raw) {
  const key = String(raw || SCAN_STATUS.NEW).toLowerCase();
  if (Object.values(SCAN_STATUS).includes(key)) return key;
  return LEGACY_STATUS[key] || SCAN_STATUS.NEW;
}

function progressForStatus(status) {
  switch (status) {
    case SCAN_STATUS.DONE:
      return 100;
    case SCAN_STATUS.PROGRESS:
      return 50;
    case SCAN_STATUS.FAIL:
    case SCAN_STATUS.CANCEL:
      return 0;
    default:
      return 0;
  }
}

function minioObjectKey(storedPath, scanId) {
  return resolveObjectKey(storedPath, scanId);
}

async function agentFetch(agentType, path, options = {}) {
  const agent = AGENTS[agentType];
  if (!agent) throw new Error(`Unknown agent type: ${agentType}`);

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch(`${agent.baseUrl}${path}`, {
      ...options,
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        ...options.headers,
      },
    });

    const text = await res.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = { raw: text };
    }

    if (!res.ok) {
      const msg =
        data?.error ||
        data?.message ||
        data?.detail ||
        `${agent.label} HTTP ${res.status}`;
      const err = new Error(msg);
      err.status = res.status;
      err.body = data;
      throw err;
    }

    return data;
  } catch (err) {
    if (err.name === "AbortError") {
      throw new Error(`${AGENTS[agentType].label} request timed out`);
    }
    if (err.cause?.code === "ECONNREFUSED" || err.message.includes("fetch failed")) {
      throw new Error(`${AGENTS[agentType].label} unreachable at ${AGENTS[agentType].baseUrl}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

function normalizeStatusResponse(agentType, data, jobId) {
  const status = normalizeStatus(data?.status);
  let progress = clampProgress(data?.progress ?? progressForStatus(status));
  if (status === SCAN_STATUS.DONE) progress = 100;

  return {
    job_id: data?.id || data?.scan_id || data?.job_id || jobId,
    status,
    progress,
    phase: data?.phase || data?.last_message || null,
    result_path: data?.result_path || data?.result_swagger_base_url || null,
    // On failure, capture the reason even when the agent reports it via
    // last_message (SAST) rather than an explicit error field, so it persists
    // for the failure-details endpoint.
    error_msg:
      data?.error_msg ||
      data?.error ||
      (status === SCAN_STATUS.FAIL ? data?.last_message || null : null),
  };
}

async function fetchHealth(agentType) {
  const agent = AGENTS[agentType];
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch(`${agent.baseUrl}${ROUTES.health}`, {
      signal: controller.signal,
      headers: { Accept: "application/json" },
    });
    const text = await res.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = { raw: text };
    }
    if (res.ok || (agentType === "sast" && res.status === 503 && data?.ping)) {
      return data;
    }
    throw new Error(`${agent.label} health HTTP ${res.status}`);
  } catch (err) {
    if (err.name === "AbortError") {
      throw new Error(`${agent.label} health request timed out`);
    }
    if (err.cause?.code === "ECONNREFUSED" || err.message.includes("fetch failed")) {
      throw new Error(`${agent.label} unreachable at ${agent.baseUrl}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

async function health(agentType) {
  const agent = AGENTS[agentType];
  try {
    const data = await fetchHealth(agentType);

    if (agentType === "sast") {
      const mysqlOk = data?.ping?.mysql === "ok";
      const minioOk = data?.ping?.minio === "ok";
      const isUp = mysqlOk && minioOk;
      return {
        agent: agentType,
        label: agent.label,
        base_url: agent.baseUrl,
        status: isUp ? "up" : "down",
        reachable: true,
        version: data?.agent || null,
        message: JSON.stringify(data?.ping || data),
        details: data,
      };
    }

    const raw = String(data?.status || "ok").toLowerCase();
    const isUp = !["down", "offline", "error", "failed", "degraded"].includes(raw);
    return {
      agent: agentType,
      label: agent.label,
      base_url: agent.baseUrl,
      status: isUp ? "up" : "down",
      reachable: true,
      version: data?.version || data?.agent || null,
      message: data?.message || data?.status || "OK",
      details: data,
    };
  } catch (err) {
    return {
      agent: agentType,
      label: agent.label,
      base_url: agent.baseUrl,
      status: "down",
      reachable: false,
      version: null,
      message: err.message,
      details: null,
    };
  }
}

async function healthAll() {
  const [sast, dast, report] = await Promise.all([
    health("sast"),
    health("dast"),
    health("report"),
  ]);
  return { sast, dast, report };
}

function buildScanBody(agentType, payload) {
  const body = {
    id: payload.job_id,
    project_id: payload.project_id,
    base_url: payload.linkapi,
  };

  if (payload.model_name) {
    body.model = payload.model_name;
  }

  if (agentType === "dast") {
    if (payload.after_sast) {
      body.swagger_path = `${payload.project_id}/sast/openapi.yaml`;
      body.sast_report_path = `${payload.project_id}/sast/report.md`;
    } else {
      body.swagger_path = minioObjectKey(payload.linkrawswagger, payload.project_id);
      body.sast_report_path = minioObjectKey(payload.linksource, payload.project_id);
    }
  }

  if (agentType === "sast" && Array.isArray(payload.modes) && payload.modes.length) {
    body.modes = payload.modes;
  }

  if (agentType === "report") {
    delete body.base_url;
  }

  return body;
}

async function run(agentType, payload) {
  const data = await agentFetch(agentType, ROUTES.run, {
    method: "POST",
    body: JSON.stringify(buildScanBody(agentType, payload)),
  });
  return normalizeStatusResponse(agentType, data, payload.job_id);
}

async function getStatus(agentType, jobId) {
  const data = await agentFetch(agentType, ROUTES.status(jobId));
  return normalizeStatusResponse(agentType, data, jobId);
}

async function cancel(agentType, jobId) {
  if (agentType === "dast") {
    return {
      job_id: jobId,
      status: SCAN_STATUS.CANCEL,
      progress: 0,
      phase: "Cancel not supported by AgentDAST",
      result_path: null,
      error_msg: null,
    };
  }
  const data = await agentFetch(agentType, `/cancel/${encodeURIComponent(jobId)}`, {
    method: "POST",
    body: JSON.stringify({ job_id: jobId }),
  });
  return normalizeStatusResponse(agentType, data, jobId);
}

function buildRunPayload({ jobId, projectId, scanRecord, modelName, after_sast = false }) {
  return {
    job_id: jobId,
    project_id: projectId,
    linksource: scanRecord.linksource,
    linkrawswagger: scanRecord.linkrawswagger,
    linkapi: scanRecord.linkapi,
    model_name: modelName || null,
    modes: parseScanModes(scanRecord.scan_modes),
    after_sast,
  };
}

// parseScanModes turns the stored CSV (e.g. "quickscan,pgwscan") into an array.
function parseScanModes(csv) {
  return String(csv || "")
    .split(",")
    .map((m) => m.trim())
    .filter(Boolean);
}

function isReachableError(err) {
  return !String(err.message || "").includes("unreachable");
}

async function testAgentConnectivity(agentType) {
  const agent = AGENTS[agentType];
  const checks = {};

  const healthResult = await health(agentType);
  checks.health = {
    ok: healthResult.reachable && healthResult.status === "up",
    ...healthResult,
  };

  const scanPayload = {
    job_id: "00000000-0000-7000-0000-000000000099",
    project_id: "00000000-0000-7000-0000-000000000001",
    linkapi: "https://example.com/v1",
    linksource: null,
    linkrawswagger: null,
  };

  try {
    const data = await agentFetch(agentType, ROUTES.run, {
      method: "POST",
      body: JSON.stringify(buildScanBody(agentType, scanPayload)),
    });
    checks.run = {
      ok: true,
      http_status: 202,
      response: data,
    };
  } catch (err) {
    checks.run = {
      ok: isReachableError(err) && [202, 400, 503].includes(err.status),
      http_status: err.status || null,
      error: err.message,
      note:
        err.status === 503
          ? "Agent reachable; run rejected (e.g. AI not configured)"
          : undefined,
    };
  }

  const probeId = "00000000-0000-7000-0000-000000000088";
  try {
    await agentFetch(agentType, ROUTES.status(probeId));
    checks.status = { ok: true, http_status: 200 };
  } catch (err) {
    checks.status = {
      ok: isReachableError(err) && err.status === 404,
      http_status: err.status || null,
      error: err.message,
      note: err.status === 404 ? "Expected 404 for unknown job id" : undefined,
    };
  }

  const ok = checks.health.ok && checks.run.ok && checks.status.ok;

  return {
    agent: agentType,
    label: agent.label,
    base_url: agent.baseUrl,
    ok,
    checks,
  };
}

async function testConnectivity() {
  const [sast, dast, report] = await Promise.all([
    testAgentConnectivity("sast"),
    testAgentConnectivity("dast"),
    testAgentConnectivity("report"),
  ]);
  return {
    ok: sast.ok && dast.ok && report.ok,
    sast,
    dast,
    report,
    tested_at: new Date().toISOString(),
  };
}

module.exports = {
  SCAN_STATUS,
  TERMINAL_STATUSES,
  AGENTS,
  ROUTES,
  health,
  healthAll,
  run,
  getStatus,
  cancel,
  buildRunPayload,
  testConnectivity,
  testAgentConnectivity,
  normalizeStatusResponse,
  normalizeStatus,
  clampProgress,
  minioObjectKey,
};
