// AgentReport generator.
//
// The LLM is asked ONLY for a compact, structured JSON object (placeholder
// values). It never emits HTML or markdown. The fixed templates in
// templates.js render that data into layout-stable, HTML-only reports.
const { renderExecutive, renderTechnical } = require("./templates");

const SEVERITY_KEYS = ["Critical", "High", "Medium", "Low", "Info"];

function extractMessageContent(data) {
  const choice = data?.choices?.[0];
  if (!choice) return "";
  const msg = choice.message || {};
  if (typeof msg.content === "string" && msg.content.trim()) return msg.content.trim();
  if (typeof msg.reasoning_content === "string" && msg.reasoning_content.trim()) {
    return msg.reasoning_content.trim();
  }
  if (Array.isArray(msg.content)) {
    const text = msg.content
      .filter((part) => part && (part.type === "text" || part.type === "output_text"))
      .map((part) => part.text || "")
      .join("")
      .trim();
    if (text) return text;
  }
  if (typeof choice.text === "string" && choice.text.trim()) return choice.text.trim();
  if (typeof msg.refusal === "string" && msg.refusal.trim()) return msg.refusal.trim();
  if (msg.parsed && typeof msg.parsed === "object") return JSON.stringify(msg.parsed);
  return "";
}

function parseJsonContent(content) {
  const trimmed = String(content || "").trim();
  if (!trimmed) throw new Error("empty JSON payload");
  try {
    return JSON.parse(trimmed);
  } catch {
    const fenced = trimmed.match(/```(?:json)?\s*([\s\S]*?)```/i);
    if (fenced) return JSON.parse(fenced[1].trim());
    // Last resort: grab the outermost {...} block.
    const start = trimmed.indexOf("{");
    const end = trimmed.lastIndexOf("}");
    if (start !== -1 && end > start) return JSON.parse(trimmed.slice(start, end + 1));
    throw new Error("LLM response is not valid JSON");
  }
}

async function requestChatCompletion({ apiBase, apiKey, model, systemPrompt, userPrompt, jsonMode }) {
  const base = String(apiBase || "").replace(/\/$/, "");
  const body = {
    model,
    temperature: 0.2,
    max_tokens: 8192,
    messages: [
      { role: "system", content: systemPrompt },
      { role: "user", content: userPrompt },
    ],
  };
  if (jsonMode) body.response_format = { type: "json_object" };

  const res = await fetch(`${base}/chat/completions`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error(`LLM invalid JSON response: ${text.slice(0, 200)}`);
  }
  if (!res.ok) {
    throw new Error(data?.error?.message || data?.error || `LLM HTTP ${res.status}`);
  }
  return {
    data,
    content: extractMessageContent(data),
    finishReason: data?.choices?.[0]?.finish_reason || null,
  };
}

async function callLlm({ apiBase, apiKey, model, systemPrompt, userPrompt }) {
  // Prefer JSON mode; fall back to plain mode for providers that ignore it.
  let result = await requestChatCompletion({
    apiBase,
    apiKey,
    model,
    systemPrompt,
    userPrompt,
    jsonMode: true,
  });

  if (!result.content) {
    result = await requestChatCompletion({
      apiBase,
      apiKey,
      model,
      systemPrompt,
      userPrompt,
      jsonMode: false,
    });
  }

  if (!result.content) {
    const reason = result.finishReason || "unknown";
    throw new Error(`LLM returned empty content (finish_reason=${reason})`);
  }

  return parseJsonContent(result.content);
}

function normalizeStats(stats) {
  const sev = {};
  const inSev = stats?.by_severity || {};
  for (const k of SEVERITY_KEYS) sev[k] = Number(inSev[k] || 0);
  const cats = Array.isArray(stats?.by_category)
    ? stats.by_category
        .map((c) => ({ name: String(c.name || c.category || "").trim(), count: Number(c.count || 0) }))
        .filter((c) => c.name)
    : [];
  return { by_severity: sev, by_category: cats };
}

function countSeverityMarkers(text) {
  const counts = { Critical: 0, High: 0, Medium: 0, Low: 0, Info: 0 };
  for (const level of Object.keys(counts)) {
    const re = new RegExp(`\\b${level}\\b`, "gi");
    counts[level] = (String(text || "").match(re) || []).length;
  }
  return counts;
}

// Builds a structured data object (NOT HTML) when the LLM is unavailable, so
// the same templates can still render a usable HTML report.
function buildFallbackData({ projectId, sastReport, dastReport }) {
  const combined = `${sastReport || ""}\n\n${dastReport || ""}`.trim();
  const stats = normalizeStats({ by_severity: countSeverityMarkers(combined), by_category: [] });
  return {
    stats,
    executive: {
      summary:
        "Automated synthesis was unavailable, so this report falls back to the raw scan output. Review the technical report for the full SAST and DAST findings.",
      risk: { level: "Info", rationale: "Risk could not be scored automatically." },
      key_findings: [],
      recommendations: ["Re-run report generation once the model is reachable."],
    },
    technical: {
      findings: [
        {
          title: "Raw SAST output",
          severity: "Info",
          source: "SAST",
          location: `${projectId}/sast/report`,
          description: "Unprocessed static analysis report.",
          evidence: (sastReport || "(no SAST report)").slice(0, 6000),
          reproduction: [],
          impact: "See evidence.",
          remediation: "Review findings individually.",
          references: [],
        },
        {
          title: "Raw DAST output",
          severity: "Info",
          source: "DAST",
          location: `${projectId}/dast/report`,
          description: "Unprocessed dynamic analysis report.",
          evidence: (dastReport || "(no DAST report)").slice(0, 6000),
          reproduction: [],
          impact: "See evidence.",
          remediation: "Review findings individually.",
          references: [],
        },
      ],
    },
    fallback: true,
  };
}

// Instruction appended to the skill prompt so the model returns ONLY the
// placeholder values for the predefined templates.
const OUTPUT_CONTRACT = `
You must return ONLY a single JSON object (no prose, no markdown, no HTML) with this exact shape:
{
  "stats": {
    "by_severity": { "Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0 },
    "by_category": [ { "name": "string", "count": 0 } ]
  },
  "executive": {
    "summary": "3-5 sentence high-level summary for management",
    "risk": { "level": "Critical|High|Medium|Low|Info", "rationale": "1-2 sentences" },
    "key_findings": [ { "title": "string", "severity": "Critical|High|Medium|Low|Info", "area": "endpoint or component" } ],
    "recommendations": [ "actionable recommendation" ]
  },
  "technical": {
    "findings": [
      {
        "title": "string",
        "severity": "Critical|High|Medium|Low|Info",
        "source": "SAST|DAST",
        "location": "file path or endpoint",
        "description": "what the issue is",
        "evidence": "short evidence excerpt from the source reports",
        "reproduction": [ "step 1", "step 2" ],
        "impact": "technical impact",
        "remediation": "concrete fix guidance",
        "references": [ "CWE-xx", "OWASP Axx" ]
      }
    ]
  }
}
Rules: use only information present in the SAST/DAST inputs; do not invent findings; deduplicate overlapping SAST/DAST issues; keep strings concise; values are plain text (the system renders them into HTML).`;

async function generateReports({ projectId, sastReport, dastReport, skillPrompt, model, apiBase, apiKey }) {
  const userPrompt = `Project ID: ${projectId}

## SAST report
${sastReport || "(empty)"}

## DAST report
${dastReport || "(empty)"}`;

  let data;
  try {
    data = await callLlm({
      apiBase,
      apiKey,
      model,
      systemPrompt: `${skillPrompt}\n\n${OUTPUT_CONTRACT}`,
      userPrompt,
    });
  } catch (err) {
    console.warn("Report LLM failed, using fallback:", err.message);
    data = buildFallbackData({ projectId, sastReport, dastReport });
  }

  const stats = normalizeStats(data.stats);
  const executive = data.executive || {};
  const technical = data.technical || {};

  const executiveHtml = renderExecutive({ projectId, executive, stats });
  const technicalHtml = renderTechnical({ projectId, technical, stats });

  return {
    stats,
    data: { stats, executive, technical, fallback: Boolean(data.fallback) },
    executive_html: executiveHtml,
    technical_html: technicalHtml,
    fallback: Boolean(data.fallback),
  };
}

// Render an HTML document to a PDF Buffer using headless Chromium.
// Charts are inline SVG so they render faithfully in the PDF.
async function htmlToPdf(html) {
  const puppeteer = require("puppeteer-core");
  const executablePath =
    process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser";

  const browser = await puppeteer.launch({
    executablePath,
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
  });
  try {
    const page = await browser.newPage();
    await page.setContent(html, { waitUntil: "networkidle0" });
    const pdf = await page.pdf({
      format: "A4",
      printBackground: true,
      margin: { top: "16mm", bottom: "16mm", left: "12mm", right: "12mm" },
    });
    return Buffer.from(pdf);
  } finally {
    await browser.close();
  }
}

module.exports = { generateReports, htmlToPdf };
