const mysql = require("mysql2/promise");

let pool;

async function initDb() {
  pool = mysql.createPool({
    host: process.env.MYSQL_HOST || "mysql",
    port: Number(process.env.MYSQL_PORT || 3306),
    user: process.env.MYSQL_USER,
    password: process.env.MYSQL_PASSWORD,
    database: process.env.MYSQL_DATABASE,
    waitForConnections: true,
    connectionLimit: 5,
  });

  await pool.execute(`
    CREATE TABLE IF NOT EXISTS report (
      id          VARCHAR(36)  PRIMARY KEY,
      project_id  VARCHAR(255) NOT NULL,
      result_path VARCHAR(512) NULL,
      status      VARCHAR(16)  NOT NULL,
      progress    TINYINT UNSIGNED NOT NULL DEFAULT 0,
      phase       VARCHAR(255) NULL,
      error_msg   TEXT         NULL,
      last_update DATETIME     NOT NULL,
      INDEX idx_report_project (project_id)
    )
  `);
}

function getPool() {
  if (!pool) throw new Error("Database not initialized");
  return pool;
}

async function getJob(id) {
  const [rows] = await getPool().execute("SELECT * FROM report WHERE id = ?", [id]);
  return rows[0] || null;
}

async function createJob(id, projectId) {
  const ts = new Date();
  await getPool().execute(
    `INSERT INTO report (id, project_id, status, progress, phase, last_update)
     VALUES (?, ?, 'new', 0, 'Queued', ?)`,
    [id, projectId, ts]
  );
}

async function updateJob(id, fields) {
  const ts = new Date();
  await getPool().execute(
    `UPDATE report SET status = ?, progress = ?, phase = ?, result_path = ?, error_msg = ?, last_update = ?
     WHERE id = ?`,
    [
      fields.status,
      fields.progress ?? 0,
      fields.phase || null,
      fields.result_path || null,
      fields.error_msg || null,
      ts,
      id,
    ]
  );
}

async function enabledModel() {
  const [rows] = await getPool().execute(
    "SELECT model_name FROM AgentModelConfig WHERE agent_type = 'report' AND enabled = 1 LIMIT 1"
  );
  return rows[0]?.model_name || process.env.OPENAI_MODEL || null;
}

module.exports = { initDb, getPool, getJob, createJob, updateJob, enabledModel };
