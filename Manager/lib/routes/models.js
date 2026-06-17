const express = require("express");
const { getPool } = require("../db");
const llm = require("../integrations/llm");
const { now, getAgentModelConfigs } = require("../services/scan");

const router = express.Router();

router.get("/", async (_req, res) => {
  try {
    const [assignments, llmHealth, remoteModels] = await Promise.all([
      getAgentModelConfigs(),
      llm.checkConnection(),
      llm.fetchRemoteModels().catch(() => []),
    ]);

    const models = llm.listAvailableModels(remoteModels);
    // SAST no longer uses the Codex Responses API — it calls standard
    // chat-completions like DAST, so any chat model is valid for SAST.
    const sastModels = models;

    res.json({
      llm_health: llmHealth,
      llm_connected: llmHealth.connected === true,
      remote_models: remoteModels,
      models,
      sast_models: sastModels,
      defaults: llm.getDefaultModels(),
      assignments,
    });
  } catch (err) {
    console.error("Failed to list models:", err);
    res.status(500).json({ error: err.message || "Failed to list models" });
  }
});

router.get("/check", async (_req, res) => {
  try {
    const health = await llm.checkConnection();
    res.json(health);
  } catch (err) {
    res.status(500).json({ error: err.message || "LLM check failed" });
  }
});

// Validates configured/advertised models against the LLM API (live probe,
// cached) and returns only those that are actually usable for chat. Used by the
// Models page to populate the assignment dropdowns with usable models only.
router.get("/usable", async (_req, res) => {
  try {
    const remoteModels = await llm.fetchRemoteModels().catch(() => []);
    const all = llm.listAvailableModels(remoteModels);
    const usable = await llm.fetchUsableModels(all);
    res.json({ usable, total: all.length, validated_at: new Date().toISOString() });
  } catch (err) {
    console.error("Model validation failed:", err);
    res.status(500).json({ error: err.message || "Model validation failed" });
  }
});

router.put("/assignments/:agentType", async (req, res) => {
  try {
    const agentType = req.params.agentType;
    if (!["sast", "dast", "report"].includes(agentType)) {
      return res.status(400).json({ error: "agent_type must be sast, dast, or report" });
    }

    const { model_name, enabled } = req.body;
    if (!model_name) {
      return res.status(400).json({ error: "model_name is required" });
    }

    // SAST now uses standard chat-completions (Codex removed), so no
    // Responses-API compatibility probe is required for any agent type.

    const pool = getPool();
    const ts = now();
    await pool.execute(
      `INSERT INTO AgentModelConfig (agent_type, model_name, litellm_model_id, enabled, updated_at)
       VALUES (?, ?, NULL, ?, ?)
       ON DUPLICATE KEY UPDATE
         model_name = VALUES(model_name),
         litellm_model_id = NULL,
         enabled = VALUES(enabled),
         updated_at = VALUES(updated_at)`,
      [agentType, model_name, enabled !== false ? 1 : 0, ts]
    );

    const [rows] = await pool.execute(
      "SELECT id, agent_type, model_name, litellm_model_id, enabled, updated_at FROM AgentModelConfig WHERE agent_type = ?",
      [agentType]
    );

    res.json(rows[0]);
  } catch (err) {
    console.error("Failed to update assignment:", err);
    res.status(500).json({ error: err.message || "Failed to update assignment" });
  }
});

router.get("/agent-config", async (_req, res) => {
  try {
    const configs = await getAgentModelConfigs();
    res.json(configs);
  } catch (err) {
    console.error("Failed to get agent config:", err);
    res.status(500).json({ error: "Failed to get agent model config" });
  }
});

module.exports = router;
