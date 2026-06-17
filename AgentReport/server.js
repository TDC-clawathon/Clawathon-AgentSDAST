const express = require("express");
const { v4: uuidv4 } = require("uuid");
const db = require("./lib/db");
const minio = require("./lib/minio");
const skills = require("./lib/skills");
const { generateReports, htmlToPdf } = require("./lib/generator");

const PORT = Number(process.env.PORT || process.env.REPORT_PORT || 8003);
const jobs = new Map();

const app = express();
app.use(express.json({ limit: "2mb" }));

app.get("/health", async (_req, res) => {
  try {
    await db.getPool().query("SELECT 1");
    res.json({ status: "ok", agent: "AgentReport", version: "1.0.0" });
  } catch (err) {
    res.status(503).json({ status: "degraded", error: err.message });
  }
});

app.get("/api/report/health", (_req, res) => {
  res.redirect(307, "/health");
});

app.get("/api/report/status", async (req, res) => {
  const id = String(req.query.id || "").trim();
  if (!id) return res.status(400).json({ error: "id is required" });

  const row = await db.getJob(id);
  if (!row) return res.status(404).json({ error: "job not found" });

  res.json({
    id: row.id,
    status: row.status,
    progress: row.progress,
    phase: row.phase,
    result_path: row.result_path,
    error_msg: row.error_msg,
  });
});

app.post("/api/report/run", async (req, res) => {
  const projectId = String(req.body.project_id || "").trim();
  if (!projectId) return res.status(400).json({ error: "project_id is required" });

  const jobId = String(req.body.id || uuidv4()).trim();
  const model = req.body.model || (await db.enabledModel());
  if (!model) return res.status(503).json({ error: "No model configured for report agent" });

  const apiKey = process.env.OPENAI_API_KEY || process.env.LLM_API_KEY;
  const apiBase = process.env.OPENAI_BASE_URL || process.env.LLM_API_BASE;
  if (!apiKey || !apiBase) {
    return res.status(503).json({ error: "LLM API not configured" });
  }

  let row = await db.getJob(jobId);
  if (!row) {
    await db.createJob(jobId, projectId);
  }

  await db.updateJob(jobId, {
    status: "progress",
    progress: 10,
    phase: "reading findings",
    result_path: null,
    error_msg: null,
  });

  res.status(202).json({
    id: jobId,
    status: "progress",
    progress: 10,
    phase: "reading findings",
  });

  if (jobs.has(jobId)) return;
  jobs.set(jobId, true);

  setImmediate(async () => {
    try {
      const sastKey = `${projectId}/sast/report.md`;
      const dastKey = `${projectId}/dast/report.md`;
      const sastReport = await minio.getText(sastKey);
      const dastReport = await minio.getText(dastKey);

      if (!sastReport && !dastReport) {
        throw new Error(`Missing both ${sastKey} and ${dastKey}`);
      }

      await db.updateJob(jobId, {
        status: "progress",
        progress: 45,
        phase: "generating report",
        result_path: null,
        error_msg: null,
      });

      const skillPrompt = await skills.loadSkillPrompt();
      // The LLM returns structured placeholder data only; templates render HTML.
      const out = await generateReports({
        projectId,
        sastReport,
        dastReport,
        skillPrompt,
        model,
        apiBase,
        apiKey,
      });

      // HTML-only artifacts. Keys keep the highlevel/detail names the Manager
      // already wires to; highlevel = Executive report, detail = Technical report.
      const prefix = `${projectId}/report`;
      await minio.putText(`${prefix}/highlevel.html`, out.executive_html, "text/html; charset=utf-8");
      await minio.putText(`${prefix}/detail.html`, out.technical_html, "text/html; charset=utf-8");
      await minio.putText(`${prefix}/stats.json`, JSON.stringify(out.stats, null, 2), "application/json");
      await minio.putText(`${prefix}/report.json`, JSON.stringify(out.data, null, 2), "application/json");

      // Render BOTH reports to PDF (Executive -> highlevel.pdf, Technical ->
      // detail.pdf). The Manager gates the Report button on both existing, so we
      // only write each once its render succeeds. A PDF failure does not fail the
      // job (the HTML stays usable), but then the button stays disabled.
      await db.updateJob(jobId, {
        status: "progress",
        progress: 80,
        phase: "rendering PDF",
        result_path: null,
        error_msg: null,
      });
      try {
        const execPdf = await htmlToPdf(out.executive_html);
        await minio.putBuffer(`${prefix}/highlevel.pdf`, execPdf, "application/pdf");
        // Back-compat alias for older callers that referenced report.pdf.
        await minio.putBuffer(`${prefix}/report.pdf`, execPdf, "application/pdf");
      } catch (pdfErr) {
        console.warn("Executive PDF generation failed (HTML still available):", pdfErr.message);
      }
      try {
        const techPdf = await htmlToPdf(out.technical_html);
        await minio.putBuffer(`${prefix}/detail.pdf`, techPdf, "application/pdf");
      } catch (pdfErr) {
        console.warn("Technical PDF generation failed (HTML still available):", pdfErr.message);
      }

      const resultPath = `${prefix}/highlevel.html`;
      await db.updateJob(jobId, {
        status: "done",
        progress: 100,
        phase: "completed",
        result_path: resultPath,
        error_msg: null,
      });
    } catch (err) {
      console.error("Report job failed:", err);
      await db.updateJob(jobId, {
        status: "fail",
        progress: 0,
        phase: "failed",
        result_path: null,
        error_msg: err.message,
      });
    } finally {
      jobs.delete(jobId);
    }
  });
});

async function main() {
  await db.initDb();
  app.listen(PORT, () => {
    console.log(`AgentReport listening on :${PORT}`);
  });
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
