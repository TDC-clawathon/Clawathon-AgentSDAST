const express = require("express");
const skills = require("../storage/skills");
const { syncAllSeededSkills } = require("../storage/syncSkills");

const router = express.Router();

router.post("/sync", async (req, res) => {
  try {
    const overwrite =
      req.body?.overwrite === true ||
      String(process.env.SKILLS_SYNC_OVERWRITE || "0").toLowerCase() === "1";
    const summary = await syncAllSeededSkills({ overwrite });
    res.json({
      overwrite,
      summary,
    });
  } catch (err) {
    console.error("Failed to sync skills:", err);
    res.status(500).json({ error: err.message || "Failed to sync skills" });
  }
});

router.get("/:agent", async (req, res) => {
  try {
    const agent = req.params.agent;
    const files = await skills.listSkills(agent);
    res.json({
      agent,
      prefix: skills.prefixFor(agent),
      files,
    });
  } catch (err) {
    console.error("Failed to list skills:", err);
    res.status(400).json({ error: err.message || "Failed to list skills" });
  }
});

router.get("/:agent/content", async (req, res) => {
  try {
    const { path } = req.query;
    if (!path) {
      return res.status(400).json({ error: "path query parameter is required" });
    }
    const file = await skills.getSkill(req.params.agent, path);
    res.json(file);
  } catch (err) {
    const status = err.name === "NoSuchKey" || err.$metadata?.httpStatusCode === 404 ? 404 : 400;
    console.error("Failed to get skill:", err);
    res.status(status).json({ error: err.message || "Failed to get skill" });
  }
});

router.put("/:agent/content", async (req, res) => {
  try {
    const { path } = req.query;
    if (!path) {
      return res.status(400).json({ error: "path query parameter is required" });
    }
    if (typeof req.body?.content !== "string") {
      return res.status(400).json({ error: "content (string) is required in body" });
    }
    const saved = await skills.putSkill(req.params.agent, path, req.body.content);
    res.json(saved);
  } catch (err) {
    console.error("Failed to save skill:", err);
    res.status(400).json({ error: err.message || "Failed to save skill" });
  }
});

router.post("/:agent/content", async (req, res) => {
  try {
    const { path } = req.query;
    if (!path) {
      return res.status(400).json({ error: "path query parameter is required" });
    }
    if (typeof req.body?.content !== "string") {
      return res.status(400).json({ error: "content (string) is required in body" });
    }

    try {
      await skills.getSkill(req.params.agent, path);
      return res.status(409).json({ error: "Skill file already exists" });
    } catch (err) {
      if (err.name !== "NoSuchKey" && err.$metadata?.httpStatusCode !== 404) {
        throw err;
      }
    }

    const saved = await skills.putSkill(req.params.agent, path, req.body.content);
    res.status(201).json(saved);
  } catch (err) {
    console.error("Failed to create skill:", err);
    res.status(400).json({ error: err.message || "Failed to create skill" });
  }
});

router.delete("/:agent/content", async (req, res) => {
  try {
    const { path } = req.query;
    if (!path) {
      return res.status(400).json({ error: "path query parameter is required" });
    }
    const removed = await skills.deleteSkill(req.params.agent, path);
    res.json(removed);
  } catch (err) {
    console.error("Failed to delete skill:", err);
    res.status(400).json({ error: err.message || "Failed to delete skill" });
  }
});

module.exports = router;
