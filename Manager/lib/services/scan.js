const { getPool } = require("../db");

function now() {
  return new Date();
}

async function getScanRecord(id) {
  const pool = getPool();
  const [rows] = await pool.execute(
    "SELECT id, linksource, linkrawswagger, linkapi, sastid, dastid, reportid, status, scan_modes, sast_model, dast_model, created_at FROM ScanRecord WHERE id = ?",
    [id]
  );
  return rows[0] || null;
}

async function hasAgentRecords(scanId) {
  const record = await getScanRecord(scanId);
  return Boolean(record?.sastid);
}

async function getAgentStatusCounts(table) {
  const pool = getPool();
  const [rows] = await pool.execute(`
    SELECT
      COUNT(*) AS total,
      SUM(status = 'new') AS new_count,
      SUM(status = 'progress') AS progress,
      SUM(status = 'done') AS done,
      SUM(status = 'fail') AS fail,
      SUM(status = 'cancel') AS cancel
    FROM ${table}
  `);
  const r = rows[0] || {};
  const total = Number(r.total || 0);
  const fail = Number(r.fail || 0);
  const progress = Number(r.progress || 0);
  const done = Number(r.done || 0);

  let health = "healthy";
  if (fail > 0 && fail >= total * 0.5) health = "critical";
  else if (progress > 0) health = "progress";
  else if (fail > 0) health = "degraded";

  return {
    total,
    new: Number(r.new_count || 0),
    progress,
    done,
    fail,
    cancel: Number(r.cancel || 0),
    health,
  };
}

async function getAgentModelConfigs() {
  const pool = getPool();
  const [rows] = await pool.execute(
    "SELECT id, agent_type, model_name, litellm_model_id, enabled, updated_at FROM AgentModelConfig ORDER BY agent_type"
  );
  return rows;
}

module.exports = {
  now,
  getScanRecord,
  hasAgentRecords,
  getAgentStatusCounts,
  getAgentModelConfigs,
};
