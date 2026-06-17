const express = require("express");
const agents = require("../agents/client");

const router = express.Router();

router.get("/health", async (_req, res) => {
  try {
    const health = await agents.healthAll();
    res.json(health);
  } catch (err) {
    console.error("Failed to get agent health:", err);
    res.status(500).json({ error: "Failed to get agent health" });
  }
});

router.get("/test", async (_req, res) => {
  try {
    const result = await agents.testConnectivity();
    res.status(result.ok ? 200 : 503).json(result);
  } catch (err) {
    console.error("Agent connectivity test failed:", err);
    res.status(500).json({ error: err.message || "Connectivity test failed" });
  }
});

module.exports = router;
