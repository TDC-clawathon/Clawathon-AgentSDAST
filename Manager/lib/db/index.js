const mysql = require("mysql2/promise");

let pool;

async function ensureColumn(table, column, definition) {
  const [rows] = await pool.execute(
    `SELECT COUNT(*) AS c FROM information_schema.COLUMNS
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
    [table, column]
  );
  if (Number(rows[0].c) === 0) {
    await pool.execute(`ALTER TABLE ${table} ADD COLUMN ${column} ${definition}`);
  }
}

// ensureColumn adds a column to an existing table if it's missing (idempotent).
async function ensureColumn(table, column, definition) {
  const [cols] = await pool.execute(
    `SELECT COUNT(*) AS c FROM information_schema.COLUMNS
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
    [table, column]
  );
  if (Number(cols[0].c) > 0) return;
  await pool.execute(`ALTER TABLE ${table} ADD COLUMN ${column} ${definition}`);
}

async function migrateScanRecordTable() {
  const [tables] = await pool.execute(
    `SELECT COUNT(*) AS c FROM information_schema.TABLES
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ScanRecord'`
  );
  if (Number(tables[0].c) === 0) return;

  const [cols] = await pool.execute(
    `SELECT DATA_TYPE FROM information_schema.COLUMNS
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ScanRecord' AND COLUMN_NAME = 'id'`
  );
  if (cols[0]?.DATA_TYPE !== "int") return;

  const [[countRow]] = await pool.execute("SELECT COUNT(*) AS c FROM ScanRecord");
  if (Number(countRow.c) > 0) {
    console.warn(
      "ScanRecord uses INT id with existing data; reset DB or migrate manually for UUIDv7"
    );
    return;
  }

  await pool.execute("DROP TABLE ScanRecord");
}

async function migrateAgentModelConfig() {
  const [tables] = await pool.execute(
    `SELECT COUNT(*) AS c FROM information_schema.TABLES
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'AgentModelConfig'`
  );
  if (Number(tables[0].c) === 0) return;

  const [cols] = await pool.execute(
    `SELECT COLUMN_TYPE FROM information_schema.COLUMNS
     WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'AgentModelConfig' AND COLUMN_NAME = 'agent_type'`
  );
  const colType = String(cols[0]?.COLUMN_TYPE || "").toLowerCase();
  if (colType.startsWith("enum")) {
    await pool.execute(
      `ALTER TABLE AgentModelConfig MODIFY agent_type VARCHAR(16) NOT NULL`
    );
  }
}

async function initDb() {
  pool = mysql.createPool({
    host: process.env.MYSQL_HOST || "localhost",
    port: Number(process.env.MYSQL_PORT || 3306),
    user: process.env.MYSQL_USER,
    password: process.env.MYSQL_PASSWORD,
    database: process.env.MYSQL_DATABASE,
    waitForConnections: true,
    connectionLimit: 10,
  });

  await migrateScanRecordTable();

  await pool.execute(`
    CREATE TABLE IF NOT EXISTS ScanRecord (
      id VARCHAR(36) PRIMARY KEY,
      linksource VARCHAR(512) NULL,
      linkrawswagger VARCHAR(512) NULL,
      linkapi VARCHAR(512) NULL,
      sastid VARCHAR(64) NULL,
      dastid VARCHAR(64) NULL,
      reportid VARCHAR(64) NULL,
      status VARCHAR(32) NOT NULL DEFAULT 'draft',
      scan_modes VARCHAR(64) NOT NULL DEFAULT 'quickscan',
      dast_triggered TINYINT(1) NOT NULL DEFAULT 0,
      report_triggered TINYINT(1) NOT NULL DEFAULT 0,
      sast_model VARCHAR(255) NULL,
      dast_model VARCHAR(255) NULL,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
  `);

  await ensureColumn("ScanRecord", "scan_modes", "VARCHAR(64) NOT NULL DEFAULT 'quickscan'");

  await pool.execute(`
    CREATE TABLE IF NOT EXISTS sast (
      id          VARCHAR(36)  PRIMARY KEY,
      project_id  VARCHAR(255) NOT NULL,
      result_path VARCHAR(512) NULL,
      status      VARCHAR(16)  NOT NULL,
      progress    TINYINT UNSIGNED NOT NULL DEFAULT 0,
      phase       VARCHAR(255) NULL,
      error_msg   TEXT         NULL,
      last_update DATETIME     NOT NULL,
      INDEX idx_sast_project (project_id)
    )
  `);

  await pool.execute(`
    CREATE TABLE IF NOT EXISTS dast (
      id          VARCHAR(36)  PRIMARY KEY,
      project_id  VARCHAR(255) NOT NULL,
      result_path VARCHAR(512) NULL,
      status      VARCHAR(16)  NOT NULL,
      progress    TINYINT UNSIGNED NOT NULL DEFAULT 0,
      phase       VARCHAR(255) NULL,
      error_msg   TEXT         NULL,
      last_update DATETIME     NOT NULL,
      INDEX idx_dast_project (project_id)
    )
  `);

  await ensureColumn("ScanRecord", "dast_triggered", "TINYINT(1) NOT NULL DEFAULT 0");
  await ensureColumn("ScanRecord", "reportid", "VARCHAR(64) NULL");
  await ensureColumn("ScanRecord", "report_triggered", "TINYINT(1) NOT NULL DEFAULT 0");
  await ensureColumn("ScanRecord", "sast_model", "VARCHAR(255) NULL");
  await ensureColumn("ScanRecord", "dast_model", "VARCHAR(255) NULL");
  await ensureColumn("sast", "progress", "TINYINT UNSIGNED NOT NULL DEFAULT 0");
  await ensureColumn("sast", "phase", "VARCHAR(255) NULL");
  await ensureColumn("sast", "last_message", "TEXT NULL");
  await ensureColumn("sast", "result_swagger_path", "VARCHAR(512) NULL");
  await ensureColumn("sast", "result_report_path", "VARCHAR(512) NULL");
  await ensureColumn("sast", "result_swagger_base_url", "VARCHAR(512) NULL");
  await ensureColumn("dast", "progress", "TINYINT UNSIGNED NOT NULL DEFAULT 0");
  await ensureColumn("dast", "phase", "VARCHAR(255) NULL");

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

  await pool.execute(`
    CREATE TABLE IF NOT EXISTS AgentModelConfig (
      id INT AUTO_INCREMENT PRIMARY KEY,
      agent_type VARCHAR(16) NOT NULL UNIQUE,
      model_name VARCHAR(255) NOT NULL,
      litellm_model_id VARCHAR(255) NULL,
      enabled TINYINT(1) NOT NULL DEFAULT 1,
      updated_at DATETIME NOT NULL
    )
  `);

  await migrateAgentModelConfig();

  const [existing] = await pool.execute("SELECT agent_type FROM AgentModelConfig");
  const types = new Set(existing.map((r) => r.agent_type));
  const ts = new Date();
  // Seed default model per agent from the environment so a clean deploy uses the
  // configured provider's models. Falls back to gpt-4o* for OpenAI-native setups.
  const seedModels = {
    sast: process.env.AGENT_SAST_MODEL || "gpt-4o",
    dast: process.env.AGENT_DAST_MODEL || "gpt-4o-mini",
    report: process.env.AGENT_REPORT_MODEL || "gpt-4o",
  };
  for (const agent of ["sast", "dast", "report"]) {
    if (!types.has(agent)) {
      await pool.execute(
        "INSERT INTO AgentModelConfig (agent_type, model_name, enabled, updated_at) VALUES (?, ?, 1, ?)",
        [agent, seedModels[agent], ts]
      );
    }
  }
}

function getPool() {
  if (!pool) {
    throw new Error("Database not initialized");
  }
  return pool;
}

module.exports = { initDb, getPool };
