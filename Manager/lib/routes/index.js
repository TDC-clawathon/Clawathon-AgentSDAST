const path = require("path");
const { requireAdmin, isAdmin } = require("../auth");
const uploadRoutes = require("./upload");
const scanRoutes = require("./scans");
const modelRoutes = require("./models");
const dashboardRoutes = require("./dashboard");
const agentRoutes = require("./agents");
const skillRoutes = require("./skills");

function registerRoutes(app) {
  // Platform health check (AgentBase Runtime polls GET /health to mark ACTIVE)
  app.get("/health", (_req, res) => res.sendStatus(200));

  // Admin portal — browser prompts for HTTP Basic Auth on first visit.
  // Serves the same SPA shell; frontend detects admin role via /api/me.
  app.get("/adm", requireAdmin, (_req, res) => {
    res.sendFile(path.join(__dirname, "../../public/index.html"));
  });

  // Role detection — never returns 401; frontend calls this to determine UI access level.
  app.get("/api/me", (req, res) => {
    res.json({ role: isAdmin(req) ? "admin" : "guest" });
  });

  // Guest-accessible routes
  app.use("/api/upload", uploadRoutes);
  app.post("/api/scan/init", scanRoutes.initScan);
  app.post("/api/scan", scanRoutes.finalizeScan);
  app.post("/api/scan/check-url", scanRoutes.checkApiUrl);
  app.use("/api/scans", scanRoutes);
  app.use("/api/dashboard", dashboardRoutes);

  // Admin-only routes — backend enforcement regardless of UI state
  app.use("/api/models", requireAdmin, modelRoutes);
  app.use("/api/agents", requireAdmin, agentRoutes);
  app.use("/api/skills", requireAdmin, skillRoutes);
}

module.exports = registerRoutes;
