const { v4: uuidv4 } = require("uuid");
const { getPool } = require("../db");
const { getScanRecord, hasAgentRecords } = require("../services/scan");
const { loadScanReports } = require("../storage/reportArtifacts");
const agents = require("./client");

const { SCAN_STATUS, TERMINAL_STATUSES } = agents;

const JOB_STATUS_ORDER_SQL = `
  CASE LOWER(status)
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
  last_update DESC
`;

function now() {
  return new Date();
}

function aggregateScanStatus(sastRow, dastRow, reportRow) {
  const sast = sastRow ? agents.normalizeStatus(sastRow.status) : null;
  const dast = dastRow ? agents.normalizeStatus(dastRow.status) : null;
  const report = reportRow ? agents.normalizeStatus(reportRow.status) : null;

  if (sast === SCAN_STATUS.FAIL || dast === SCAN_STATUS.FAIL || report === SCAN_STATUS.FAIL) {
    return SCAN_STATUS.FAIL;
  }
  if (sast === SCAN_STATUS.CANCEL || dast === SCAN_STATUS.CANCEL || report === SCAN_STATUS.CANCEL) {
    return SCAN_STATUS.CANCEL;
  }
  if (sast === SCAN_STATUS.DONE && dast === SCAN_STATUS.DONE && report === SCAN_STATUS.DONE) {
    return SCAN_STATUS.DONE;
  }
  if (
    sast === SCAN_STATUS.PROGRESS ||
    dast === SCAN_STATUS.PROGRESS ||
    report === SCAN_STATUS.PROGRESS ||
    (sast === SCAN_STATUS.DONE && dast === SCAN_STATUS.NEW) ||
    (sast === SCAN_STATUS.DONE && dast === SCAN_STATUS.DONE && report === SCAN_STATUS.NEW)
  ) {
    return SCAN_STATUS.PROGRESS;
  }
  if (sast === SCAN_STATUS.NEW) return SCAN_STATUS.PROGRESS;
  return SCAN_STATUS.NEW;
}

function normalizeJobRow(table, row) {
  if (!row) return null;
  const status = agents.normalizeStatus(row.status);
  let progress = Number(row.progress || 0);
  if (status === agents.SCAN_STATUS.DONE) {
    progress = 100;
  } else if (!progress) {
    progress =
      status === agents.SCAN_STATUS.PROGRESS
        ? 50
        : 0;
  }
  const phase = row.phase || row.last_message || row.error_msg || null;

  if (table === "sast") {
    return {
      ...row,
      status,
      progress,
      phase,
      result_path: row.result_swagger_base_url || row.result_swagger_path || null,
      error_msg: row.last_message || row.error_msg || null,
    };
  }

  return {
    ...row,
    status,
    progress,
    phase,
    result_path: row.result_path || null,
    error_msg: row.error_msg || null,
  };
}

async function getLatestJobRow(table, scanId) {
  const pool = getPool();
  const [rows] = await pool.execute(
    `SELECT * FROM ${table} WHERE project_id = ? ORDER BY ${JOB_STATUS_ORDER_SQL} LIMIT 1`,
    [scanId]
  );
  return normalizeJobRow(table, rows[0] || null);
}

// Report jobs can stall in DB when agent-report restarts mid-PDF (in-memory job lost).
async function reconcileReportFromArtifacts(scanId, reportRow) {
  if (!reportRow?.id) return reportRow;
  const status = agents.normalizeStatus(reportRow.status);
  if (TERMINAL_STATUSES.has(status)) return reportRow;

  const artifacts = await loadScanReports(scanId);
  if (!artifacts.ready) return reportRow;

  const resultPath = `${scanId}/report/highlevel.html`;
  await updateJobRow("report", reportRow.id, {
    status: SCAN_STATUS.DONE,
    progress: 100,
    phase: "completed",
    result_path: resultPath,
    error_msg: null,
  });
  return getJobRow("report", reportRow.id);
}

async function resolveJobId(table, scanId, storedJobId) {
  const latest = await getLatestJobRow(table, scanId);
  if (latest?.id) return latest.id;
  if (storedJobId) {
    const row = await getJobRow(table, storedJobId);
    if (row) return storedJobId;
  }
  return storedJobId || null;
}

async function syncScanJobIds(scanId, record = null) {
  const rec = record || (await getScanRecord(scanId));
  if (!rec) return null;

  const sastid = await resolveJobId("sast", scanId, rec.sastid);
  const dastid = await resolveJobId("dast", scanId, rec.dastid);
  const reportid = await resolveJobId("report", scanId, rec.reportid);

  if (sastid !== rec.sastid || dastid !== rec.dastid || reportid !== rec.reportid) {
    const pool = getPool();
    await pool.execute(
      "UPDATE ScanRecord SET sastid = ?, dastid = ?, reportid = ? WHERE id = ?",
      [sastid, dastid, reportid, scanId]
    );
    rec.sastid = sastid;
    rec.dastid = dastid;
    rec.reportid = reportid;
  }

  return rec;
}

async function getJobRow(table, jobId) {
  const pool = getPool();
  const [rows] = await pool.execute(`SELECT * FROM ${table} WHERE id = ?`, [jobId]);
  return normalizeJobRow(table, rows[0] || null);
}

async function updateJobRow(table, jobId, fields) {
  const pool = getPool();
  const ts = now();
  const status = agents.normalizeStatus(fields.status);

  if (table === "sast") {
    const [result] = await pool.execute(
      `UPDATE sast
       SET status = ?, progress = ?, phase = ?, last_message = ?, last_update = ?
       WHERE id = ?`,
      [
        status,
        fields.progress,
        fields.phase,
        fields.error_msg || fields.phase || null,
        ts,
        jobId,
      ]
    );
    if (result.affectedRows === 0) return;

    if (fields.result_path) {
      await pool.execute(
        `UPDATE sast SET result_swagger_base_url = ? WHERE id = ?`,
        [fields.result_path, jobId]
      );
    }
    return;
  }

  await pool.execute(
    `UPDATE ${table}
     SET status = ?, progress = ?, phase = ?, result_path = ?, error_msg = ?, last_update = ?
     WHERE id = ?`,
    [
      status,
      fields.progress,
      fields.phase,
      fields.result_path,
      fields.error_msg,
      ts,
      jobId,
    ]
  );
}

function jobTable(agentType) {
  if (agentType === "sast") return "sast";
  if (agentType === "dast") return "dast";
  if (agentType === "report") return "report";
  throw new Error(`Unknown agent type: ${agentType}`);
}

async function syncJobFromAgent(agentType, jobId) {
  const table = jobTable(agentType);
  const row = await getJobRow(table, jobId);
  if (!row) return null;

  if (TERMINAL_STATUSES.has(agents.normalizeStatus(row.status))) {
    return {
      agent: agentType,
      job_id: jobId,
      status: agents.normalizeStatus(row.status),
      progress: row.progress,
      phase: row.phase,
      result_path: row.result_path,
      error_msg: row.error_msg,
      synced: false,
      cached: true,
    };
  }

  try {
    const remote = await agents.getStatus(agentType, jobId);
    await updateJobRow(table, jobId, {
      status: remote.status,
      progress: remote.progress,
      phase: remote.phase,
      result_path: remote.result_path,
      error_msg: remote.error_msg,
    });
    return { ...remote, agent: agentType, synced: true, cached: false };
  } catch (err) {
    return {
      agent: agentType,
      job_id: jobId,
      status: agents.normalizeStatus(row.status),
      progress: row.progress,
      phase: row.phase,
      result_path: row.result_path,
      error_msg: row.error_msg,
      synced: false,
      cached: true,
      sync_error: err.message,
    };
  }
}

async function refreshScanRecordStatus(scanId) {
  const pool = getPool();
  const record = await syncScanJobIds(scanId);
  if (!record) return null;

  let sastRow = null;
  let dastRow = null;
  let reportRow = null;

  if (record.sastid) {
    await syncJobFromAgent("sast", record.sastid);
    sastRow = await getJobRow("sast", record.sastid);
  } else {
    sastRow = await getLatestJobRow("sast", scanId);
  }

  if (record.dastid) {
    await syncJobFromAgent("dast", record.dastid);
  }
  dastRow =
    (record.dastid && (await getJobRow("dast", record.dastid))) ||
    (await getLatestJobRow("dast", scanId));

  if (record.reportid) {
    await syncJobFromAgent("report", record.reportid);
  }
  reportRow =
    (record.reportid && (await getJobRow("report", record.reportid))) ||
    (await getLatestJobRow("report", scanId));
  reportRow = await reconcileReportFromArtifacts(scanId, reportRow);

  if (
    !dastRow &&
    sastRow &&
    agents.normalizeStatus(sastRow.status) === SCAN_STATUS.DONE
  ) {
    dastRow = {
      id: record.dastid,
      status: SCAN_STATUS.NEW,
      progress: 0,
      phase: "Waiting for DAST",
    };
  }

  if (
    !reportRow &&
    dastRow &&
    agents.normalizeStatus(dastRow.status) === SCAN_STATUS.DONE
  ) {
    reportRow = {
      id: record.reportid,
      status: SCAN_STATUS.NEW,
      progress: 0,
      phase: "Waiting for Report",
    };
  }

  if (sastRow || dastRow || reportRow) {
    const prevStatus = agents.normalizeStatus(record.status);
    const nextStatus = aggregateScanStatus(sastRow, dastRow, reportRow);
    if (nextStatus !== record.status) {
      await pool.execute("UPDATE ScanRecord SET status = ? WHERE id = ?", [nextStatus, scanId]);
      record.status = nextStatus;
    }
    if (
      TERMINAL_STATUSES.has(nextStatus) &&
      prevStatus === SCAN_STATUS.PROGRESS
    ) {
      await tryDequeueNextScan();
    }
  }

  await tryStartDastJob(scanId);
  await tryStartReportJob(scanId);

  return { record, sast: sastRow, dast: dastRow, report: reportRow };
}

async function syncAllRunningJobs() {
  const pool = getPool();
  const [sastJobs] = await pool.execute(
    "SELECT id, project_id FROM sast WHERE LOWER(status) IN ('new', 'progress', 'process')"
  );
  const [dastJobs] = await pool.execute(
    "SELECT id, project_id FROM dast WHERE LOWER(status) IN ('new', 'progress', 'processing', 'process')"
  );
  const [reportJobs] = await pool.execute(
    "SELECT id, project_id FROM report WHERE LOWER(status) IN ('new', 'progress', 'processing', 'process')"
  );

  const scanIds = new Set([
    ...sastJobs.map((j) => j.project_id),
    ...dastJobs.map((j) => j.project_id),
    ...reportJobs.map((j) => j.project_id),
  ]);

  for (const job of sastJobs) {
    await syncJobFromAgent("sast", job.id);
  }
  for (const job of dastJobs) {
    await syncJobFromAgent("dast", job.id);
  }
  for (const job of reportJobs) {
    await syncJobFromAgent("report", job.id);
  }

  const [pendingDast] = await pool.execute(`
    SELECT sr.id
    FROM ScanRecord sr
    JOIN sast s ON s.project_id = sr.id
    WHERE s.status = 'done'
      AND sr.dast_triggered = 0
      AND NOT EXISTS (
        SELECT 1 FROM dast d
        WHERE d.project_id = sr.id AND d.status NOT IN ('new')
      )
  `);
  for (const row of pendingDast) {
    scanIds.add(row.id);
  }

  const [pendingReport] = await pool.execute(`
    SELECT sr.id
    FROM ScanRecord sr
    JOIN dast d ON d.project_id = sr.id
    WHERE d.status = 'done'
      AND sr.report_triggered = 0
      AND NOT EXISTS (
        SELECT 1 FROM report r
        WHERE r.project_id = sr.id AND r.status NOT IN ('new')
      )
  `);
  for (const row of pendingReport) {
    scanIds.add(row.id);
  }

  const [openScans] = await pool.execute(
    `SELECT id FROM ScanRecord WHERE LOWER(status) = 'progress'`
  );
  for (const row of openScans) {
    scanIds.add(row.id);
  }

  for (const scanId of scanIds) {
    await refreshScanRecordStatus(scanId);
  }

  await tryDequeueNextScan();
}

async function getActiveScanId() {
  const pool = getPool();
  const [rows] = await pool.execute(
    `SELECT id FROM ScanRecord WHERE status = 'progress' LIMIT 1`
  );
  return rows[0]?.id || null;
}

async function enqueueScan(scanId) {
  const pool = getPool();
  await pool.execute(`UPDATE ScanRecord SET status = 'queued' WHERE id = ? AND status = 'new'`, [
    scanId,
  ]);
  return { ok: true, status: "queued", queued: true, id: scanId };
}

async function startScanPipeline(scanId) {
  const record = await getScanRecord(scanId);
  if (!record) {
    return { ok: false, error: "Scan record not found" };
  }

  const status = agents.normalizeStatus(record.status);
  if (status !== SCAN_STATUS.NEW && status !== SCAN_STATUS.QUEUED) {
    return { ok: false, error: `Cannot start scan in status ${record.status}` };
  }

  if (await hasAgentRecords(scanId)) {
    return { ok: false, error: "Scan already started for this record" };
  }

  const pool = getPool();

  // Persist both models immediately (COALESCE keeps original values on re-entry when
  // dequeued). This ensures labels are visible right after the user clicks Run,
  // even while the scan is waiting in the queue.
  const [sastModelEarly, dastModelEarly] = await Promise.all([
    getModelName("sast"),
    getModelName("dast"),
  ]);
  if (sastModelEarly || dastModelEarly) {
    await pool.execute(
      "UPDATE ScanRecord SET sast_model = COALESCE(sast_model, ?), dast_model = COALESCE(dast_model, ?) WHERE id = ?",
      [sastModelEarly || null, dastModelEarly || null, scanId]
    );
  }

  const conn = await pool.getConnection();
  let claimed = false;

  try {
    await conn.beginTransaction();

    const [active] = await conn.execute(
      `SELECT id FROM ScanRecord WHERE status = 'progress' LIMIT 1 FOR UPDATE`
    );

    if (active.length > 0 && active[0].id !== scanId) {
      await conn.commit();
      if (status === SCAN_STATUS.NEW) {
        return enqueueScan(scanId);
      }
      return { ok: true, status: SCAN_STATUS.QUEUED, queued: true, id: scanId };
    }

    const [claim] = await conn.execute(
      `UPDATE ScanRecord SET status = 'progress'
       WHERE id = ?
         AND status IN ('new', 'queued')`,
      [scanId]
    );

    if (claim.affectedRows === 0) {
      await conn.commit();
      if (status === SCAN_STATUS.NEW) {
        return enqueueScan(scanId);
      }
      if (status === SCAN_STATUS.QUEUED) {
        return { ok: true, status: SCAN_STATUS.QUEUED, queued: true, id: scanId };
      }
      return { ok: false, error: `Cannot claim scan in status ${record.status}` };
    }

    claimed = true;
  } catch (err) {
    try {
      await conn.rollback();
    } catch (_) {}
    throw err;
  } finally {
    conn.release();
  }

  const sastId = uuidv4();
  const dastId = uuidv4();
  const reportId = uuidv4();
  const ts = now();

  try {
    await conn.beginTransaction();

    await conn.execute(
      `INSERT INTO dast (id, project_id, status, progress, phase, last_update)
       VALUES (?, ?, 'new', 0, 'Queued until SAST completes', ?)`,
      [dastId, scanId, ts]
    );

    await conn.execute(
      `INSERT INTO report (id, project_id, status, progress, phase, last_update)
       VALUES (?, ?, 'new', 0, 'Queued until DAST completes', ?)`,
      [reportId, scanId, ts]
    );

    await conn.execute(
      `UPDATE ScanRecord SET sastid = ?, dastid = ?, reportid = ?, dast_triggered = 0, report_triggered = 0, status = 'progress' WHERE id = ?`,
      [sastId, dastId, reportId, scanId]
    );

    await conn.commit();
  } catch (err) {
    await conn.rollback();
    throw err;
  } finally {
    conn.release();
  }

  const freshRecord = await getScanRecord(scanId);
  // Use the model captured at run time (stored above via COALESCE).
  const sastModel = freshRecord.sast_model || sastModelEarly;
  const sastRun = await startAgentJob("sast", sastId, freshRecord, sastModel);

  await refreshScanRecordStatus(scanId);
  const updated = await getScanRecord(scanId);

  return {
    ok: true,
    id: scanId,
    sastid: updated?.sastid || sastId,
    dastid: updated?.dastid || dastId,
    reportid: updated?.reportid || reportId,
    status: "progress",
    can_run_scan: false,
    workflow: "sast_started",
    agent_results: {
      sast: sastRun,
      dast: { status: "new", message: "Queued until SAST completes" },
      report: { status: "new", message: "Queued until DAST completes" },
    },
  };
}

async function tryDequeueNextScan() {
  if (await getActiveScanId()) {
    return null;
  }

  const pool = getPool();
  const [rows] = await pool.execute(
    `SELECT id FROM ScanRecord WHERE status = 'queued' ORDER BY created_at ASC LIMIT 1`
  );
  if (!rows.length) {
    return null;
  }

  return startScanPipeline(rows[0].id);
}

async function tryStartDastJob(scanId) {
  const record = await syncScanJobIds(scanId);
  if (!record?.sastid) return null;

  const sastRow =
    (await getJobRow("sast", record.sastid)) || (await getLatestJobRow("sast", scanId));
  if (!sastRow || agents.normalizeStatus(sastRow.status) !== SCAN_STATUS.DONE) return null;

  const pool = getPool();

  const [alreadyRunning] = await pool.execute(
    `SELECT id FROM dast WHERE project_id = ? AND LOWER(status) NOT IN ('new') LIMIT 1`,
    [scanId]
  );
  if (alreadyRunning.length > 0) {
    const runningId = alreadyRunning[0].id;
    if (record.dastid !== runningId) {
      await pool.execute(`UPDATE ScanRecord SET dastid = ? WHERE id = ?`, [runningId, scanId]);
      record.dastid = runningId;
    }
    return null;
  }

  const [claim] = await pool.execute(
    `UPDATE ScanRecord SET dast_triggered = 1
     WHERE id = ? AND dast_triggered = 0`,
    [scanId]
  );
  if (claim.affectedRows === 0) return null;

  const dastJobId =
    record.dastid ||
    (await getLatestJobRow("dast", scanId))?.id ||
    null;
  if (!dastJobId) return null;

  await pool.execute(
    `UPDATE dast SET status = 'progress', phase = 'Starting DAST', progress = 0, last_update = ?
     WHERE id = ? AND status = 'new'`,
    [now(), dastJobId]
  );

  // Use the model captured at scan run time; fall back to current config for pre-v6.2 scans.
  const dastModel = record.dast_model || await getModelName("dast");
  if (dastModel && !record.dast_model) {
    await pool.execute("UPDATE ScanRecord SET dast_model = ? WHERE id = ?", [dastModel, scanId]);
  }
  return startAgentJob("dast", dastJobId, record, dastModel, { after_sast: true });
}

async function tryStartReportJob(scanId) {
  const record = await syncScanJobIds(scanId);
  if (!record?.dastid) return null;

  const dastRow =
    (await getJobRow("dast", record.dastid)) || (await getLatestJobRow("dast", scanId));
  if (!dastRow || agents.normalizeStatus(dastRow.status) !== SCAN_STATUS.DONE) return null;

  const pool = getPool();

  const [alreadyRunning] = await pool.execute(
    `SELECT id FROM report WHERE project_id = ? AND status NOT IN ('new', 'fail') LIMIT 1`,
    [scanId]
  );
  if (alreadyRunning.length > 0) return null;

  const failedReport = await getLatestJobRow("report", scanId);
  if (failedReport && agents.normalizeStatus(failedReport.status) === SCAN_STATUS.FAIL) {
    await pool.execute(
      `UPDATE report SET status = 'new', progress = 0, phase = 'Queued', error_msg = NULL, last_update = ?
       WHERE id = ?`,
      [now(), failedReport.id]
    );
    await pool.execute(`UPDATE ScanRecord SET report_triggered = 0 WHERE id = ?`, [scanId]);
  }

  const [claim] = await pool.execute(
    `UPDATE ScanRecord SET report_triggered = 1
     WHERE id = ? AND report_triggered = 0`,
    [scanId]
  );
  if (claim.affectedRows === 0) return null;

  const reportJobId =
    record.reportid || (await getLatestJobRow("report", scanId))?.id || null;
  if (!reportJobId) return null;

  await pool.execute(
    `UPDATE report SET status = 'progress', phase = 'Starting Report', progress = 0, last_update = ?
     WHERE id = ? AND status = 'new'`,
    [now(), reportJobId]
  );

  const reportModel = await getModelName("report");
  return startAgentJob("report", reportJobId, record, reportModel);
}

async function rerunReportJob(scanId) {
  const record = await syncScanJobIds(scanId);
  if (!record) {
    return { ok: false, error: "Scan record not found" };
  }

  const dastRow =
    (record.dastid && (await getJobRow("dast", record.dastid))) ||
    (await getLatestJobRow("dast", scanId));
  if (!dastRow || agents.normalizeStatus(dastRow.status) !== SCAN_STATUS.DONE) {
    return { ok: false, error: "DAST must be done before regenerating report" };
  }

  const reportRow = await getLatestJobRow("report", scanId);
  const reportJobId = record.reportid || reportRow?.id;
  if (!reportJobId) {
    return { ok: false, error: "No report job for this scan" };
  }

  const pool = getPool();
  await pool.execute(
    `UPDATE report SET status = 'new', progress = 0, phase = 'Queued', result_path = NULL, error_msg = NULL, last_update = ?
     WHERE id = ?`,
    [now(), reportJobId]
  );
  await pool.execute(`UPDATE ScanRecord SET report_triggered = 0, status = 'progress' WHERE id = ?`, [
    scanId,
  ]);

  const result = await tryStartReportJob(scanId);
  if (!result || result.ok === false) {
    return { ok: false, error: result?.error || "Failed to start report job" };
  }
  return { ok: true, job_id: result.job_id, status: result.status };
}

async function startAgentJob(agentType, jobId, scanRecord, modelName, options = {}) {
  const table = jobTable(agentType);
  const projectId = String(scanRecord.id);
  const payload = agents.buildRunPayload({
    jobId,
    projectId,
    scanRecord,
    modelName,
    after_sast: options.after_sast === true,
  });

  const idColumn =
    agentType === "sast" ? "sastid" : agentType === "dast" ? "dastid" : "reportid";

  try {
    const remote = await agents.run(agentType, payload);
    const actualJobId = remote.job_id || jobId;

    if (actualJobId !== jobId) {
      const pool = getPool();
      await pool.execute(`UPDATE ScanRecord SET ${idColumn} = ? WHERE id = ?`, [
        actualJobId,
        scanRecord.id,
      ]);
      await pool.execute(`DELETE FROM ${table} WHERE id = ? AND project_id = ?`, [
        jobId,
        projectId,
      ]);
    }

    await updateJobRow(table, actualJobId, {
      status: remote.status,
      progress: remote.progress,
      phase: remote.phase || "Started",
      result_path: remote.result_path,
      error_msg: remote.error_msg,
    });
    return { ok: true, ...remote, job_id: actualJobId };
  } catch (err) {
    const existing = await getJobRow(table, jobId);
    if (existing) {
      await updateJobRow(table, jobId, {
        status: SCAN_STATUS.FAIL,
        progress: 0,
        phase: "Start failed",
        result_path: null,
        error_msg: err.message,
      });
    }
    return { ok: false, error: err.message };
  }
}

async function cancelAgentJob(agentType, jobId) {
  const table = jobTable(agentType);

  if (agentType === "report" || agentType === "dast") {
    await updateJobRow(table, jobId, {
      status: SCAN_STATUS.CANCEL,
      progress: 0,
      phase: "Cancelled (local)",
      result_path: null,
      error_msg: null,
    });
    return { ok: true, job_id: jobId, status: SCAN_STATUS.CANCEL };
  }

  try {
    const remote = await agents.cancel(agentType, jobId);
    await updateJobRow(table, jobId, {
      status: SCAN_STATUS.CANCEL,
      progress: remote.progress,
      phase: remote.phase || "Cancelled",
      result_path: remote.result_path,
      error_msg: null,
    });
    return { ok: true, ...remote };
  } catch (err) {
    await updateJobRow(table, jobId, {
      status: SCAN_STATUS.CANCEL,
      progress: 0,
      phase: "Cancelled (local)",
      result_path: null,
      error_msg: err.message,
    });
    return { ok: false, error: err.message };
  }
}

async function getModelName(agentType) {
  const pool = getPool();
  const [rows] = await pool.execute(
    "SELECT model_name FROM AgentModelConfig WHERE agent_type = ? AND enabled = 1",
    [agentType]
  );
  return rows[0]?.model_name || null;
}

function startBackgroundSync(intervalMs = 5000) {
  const tick = () => {
    syncAllRunningJobs().catch((err) => {
      console.warn("Background agent sync failed:", err.message);
    });
  };
  tick();
  return setInterval(tick, intervalMs);
}

module.exports = {
  aggregateScanStatus,
  syncJobFromAgent,
  refreshScanRecordStatus,
  syncAllRunningJobs,
  getActiveScanId,
  startScanPipeline,
  tryDequeueNextScan,
  tryStartDastJob,
  tryStartReportJob,
  rerunReportJob,
  startAgentJob,
  cancelAgentJob,
  getModelName,
  startBackgroundSync,
  syncScanJobIds,
  getLatestJobRow,
};
