require("dotenv").config({ path: require("path").join(__dirname, "..", ".env") });

const express = require("express");
const cors = require("cors");
const path = require("path");
const fs = require("fs");
const crypto = require("crypto");
const { initDb } = require("./lib/db");
const orchestrator = require("./lib/agents/orchestrator");
const { syncOnStartup } = require("./lib/storage/syncSkills");
const registerRoutes = require("./lib/routes");

const app = express();
const PORT = process.env.MANAGER_PORT || 8000;

app.use(cors());
app.use(express.json());
app.use(express.static(path.join(__dirname, "public")));
registerRoutes(app);

// Generate and persist admin credentials if not already configured.
// Preserves existing credentials on upgrades; never overwrites.
function ensureAdminCredentials() {
  if (process.env.ADMIN_PASSWORD) return;
  const envPath = path.join(__dirname, "..", ".env");
  const password = crypto.randomBytes(12).toString("hex");
  const username = process.env.ADMIN_USERNAME || "admin";
  const lines = [];
  if (!process.env.ADMIN_USERNAME) lines.push(`ADMIN_USERNAME=${username}`);
  lines.push(`ADMIN_PASSWORD=${password}`);
  try {
    fs.appendFileSync(envPath, "\n" + lines.join("\n") + "\n");
    process.env.ADMIN_PASSWORD = password;
    if (!process.env.ADMIN_USERNAME) process.env.ADMIN_USERNAME = username;
    console.log(`[auth] Admin credentials generated — ADMIN_USERNAME=${username} ADMIN_PASSWORD=${password}`);
  } catch (err) {
    process.env.ADMIN_PASSWORD = password;
    if (!process.env.ADMIN_USERNAME) process.env.ADMIN_USERNAME = username;
    console.warn(`[auth] Could not persist to .env (${err.message}) — credentials valid for this process only`);
  }
}

async function start() {
  ensureAdminCredentials();
  await initDb();
  await syncOnStartup();
  orchestrator.startBackgroundSync(Number(process.env.AGENT_SYNC_INTERVAL_MS || 5000));
  app.listen(PORT, () => {
    console.log(`Manager running at http://localhost:${PORT}`);
  });
}

start().catch((err) => {
  console.error("Failed to start server:", err);
  process.exit(1);
});
