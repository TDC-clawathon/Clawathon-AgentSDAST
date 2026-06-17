const express = require("express");
const { getPool } = require("../db");
const llm = require("../integrations/llm");
const dataplane = require("../integrations/dataplane");
const agents = require("../agents/client");
const {
  getAgentStatusCounts,
  getAgentModelConfigs,
} = require("../services/scan");

const router = express.Router();

const EMPTY_SCANS = {
  total: 0,
  new: 0,
  progress: 0,
  done: 0,
  fail: 0,
  cancel: 0,
};

async function loadDbSummary() {
  const pool = getPool();

  const [[scanTotals]] = await pool.execute(`
    SELECT
      COUNT(*) AS total,
      SUM(status = 'new') AS new_count,
      SUM(status = 'progress') AS progress,
      SUM(status = 'done') AS done,
      SUM(status = 'fail') AS fail,
      SUM(status = 'cancel') AS cancel
    FROM ScanRecord
    WHERE status != 'draft'
  `);

  const [recentScans] = await pool.execute(`
    SELECT id, linkapi, status, sastid, dastid, created_at
    FROM ScanRecord
    WHERE status != 'draft'
    ORDER BY created_at DESC
    LIMIT 10
  `);

  const [statusBreakdown] = await pool.execute(`
    SELECT status, COUNT(*) AS count
    FROM ScanRecord
    GROUP BY status
    ORDER BY count DESC
  `);

  const [sastHealth, dastHealth, reportHealth, assignments] = await Promise.all([
    getAgentStatusCounts("sast"),
    getAgentStatusCounts("dast"),
    getAgentStatusCounts("report"),
    getAgentModelConfigs(),
  ]);

  return {
    scans: {
      total: Number(scanTotals.total || 0),
      new: Number(scanTotals.new_count || 0),
      progress: Number(scanTotals.progress || 0),
      done: Number(scanTotals.done || 0),
      fail: Number(scanTotals.fail || 0),
      cancel: Number(scanTotals.cancel || 0),
    },
    agents: { sast: sastHealth, dast: dastHealth, report: reportHealth },
    assignments,
    status_breakdown: statusBreakdown,
    recent_scans: recentScans,
  };
}

router.get("/summary", async (_req, res) => {
  try {
    const [infrastructure, llmHealth, agentHealth] = await Promise.all([
      dataplane.checkAll(),
      llm.checkConnection(),
      agents.healthAll(),
    ]);

    let dbSummary = null;
    let dbError = null;
    if (infrastructure.mysql.reachable) {
      try {
        dbSummary = await loadDbSummary();
      } catch (err) {
        dbError = err.message || "Database query failed";
      }
    }

    const assignments = dbSummary?.assignments || [];
    const assignmentMap = Object.fromEntries(
      assignments.map((a) => [a.agent_type, a.model_name])
    );

    const sast = dbSummary?.agents.sast || { total: 0, progress: 0, health: "healthy" };
    const dast = dbSummary?.agents.dast || { total: 0, progress: 0, health: "healthy" };
    const report = dbSummary?.agents.report || { total: 0, progress: 0, health: "healthy" };

    res.json({
      scans: dbSummary?.scans || EMPTY_SCANS,
      agents: {
        sast: { ...sast, service: agentHealth.sast },
        dast: { ...dast, service: agentHealth.dast },
        report: { ...report, service: agentHealth.report },
      },
      infrastructure,
      mysql: infrastructure.mysql,
      minio: infrastructure.minio,
      db_error: dbError,
      llm: {
        ...llmHealth,
        defaults: llm.getDefaultModels(),
      },
      model_assignments: assignmentMap,
      status_breakdown: dbSummary?.status_breakdown || [],
      recent_scans: dbSummary?.recent_scans || [],
    });
  } catch (err) {
    console.error("Failed to load dashboard summary:", err);
    res.status(500).json({ error: "Failed to load dashboard summary" });
  }
});

module.exports = router;
