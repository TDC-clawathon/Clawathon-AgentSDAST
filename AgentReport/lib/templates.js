// Predefined HTML report templates.
//
// The LLM never produces HTML. It returns a compact, structured JSON object
// (see generator.js) and these functions render that data into fixed,
// layout-stable HTML. This keeps rendering consistent across viewers/PDF and
// minimizes token usage (the model only fills placeholder values).

function escapeHtml(s) {
  return String(s == null ? "" : s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

const SEVERITY_ORDER = ["Critical", "High", "Medium", "Low", "Info"];
const SEVERITY_COLORS = {
  Critical: "#dc2626",
  High: "#ea580c",
  Medium: "#d97706",
  Low: "#16a34a",
  Info: "#64748b",
};

function sevClass(severity) {
  const s = String(severity || "").toLowerCase();
  if (s.startsWith("crit")) return "sev-critical";
  if (s.startsWith("high")) return "sev-high";
  if (s.startsWith("med")) return "sev-medium";
  if (s.startsWith("low")) return "sev-low";
  return "sev-info";
}

/* ---------------- Inline SVG charts (render in HTML view and PDF) ---------------- */
function polarToCartesian(cx, cy, r, angleDeg) {
  const a = ((angleDeg - 90) * Math.PI) / 180;
  return { x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) };
}

function arcPath(cx, cy, r, startAngle, endAngle) {
  const start = polarToCartesian(cx, cy, r, endAngle);
  const end = polarToCartesian(cx, cy, r, startAngle);
  const largeArc = endAngle - startAngle <= 180 ? "0" : "1";
  return `M ${cx} ${cy} L ${start.x.toFixed(2)} ${start.y.toFixed(2)} A ${r} ${r} 0 ${largeArc} 0 ${end.x.toFixed(2)} ${end.y.toFixed(2)} Z`;
}

function buildSeverityPie(severity) {
  const entries = SEVERITY_ORDER.map((k) => ({
    label: k,
    value: Number(severity[k] || 0),
    color: SEVERITY_COLORS[k],
  })).filter((e) => e.value > 0);
  const total = entries.reduce((s, e) => s + e.value, 0);
  if (!total) return `<div class="chart-empty">No severity data</div>`;

  const cx = 90;
  const cy = 90;
  const r = 82;
  let angle = 0;
  const slices = entries
    .map((e) => {
      const sweep = (e.value / total) * 360;
      let shape;
      if (entries.length === 1) {
        shape = `<circle cx="${cx}" cy="${cy}" r="${r}" fill="${e.color}"/>`;
      } else {
        shape = `<path d="${arcPath(cx, cy, r, angle, angle + sweep)}" fill="${e.color}"/>`;
      }
      angle += sweep;
      return shape;
    })
    .join("");
  const legend = entries
    .map(
      (e) =>
        `<div class="legend-item"><span class="legend-swatch" style="background:${e.color}"></span>${escapeHtml(e.label)} <strong>${e.value}</strong> <span class="legend-pct">(${Math.round((e.value / total) * 100)}%)</span></div>`
    )
    .join("");

  return `<div class="chart pie-chart">
    <svg viewBox="0 0 180 180" width="180" height="180" role="img" aria-label="Severity distribution">${slices}</svg>
    <div class="legend">${legend}</div>
  </div>`;
}

function buildCategoryColumns(categories) {
  const data = (categories || [])
    .map((c) => ({ label: String(c.name || c.category || "?"), value: Number(c.count || 0) }))
    .filter((d) => d.value > 0);
  if (!data.length) return `<div class="chart-empty">No category data</div>`;

  const max = Math.max(1, ...data.map((d) => d.value));
  const barW = 48;
  const gap = 22;
  const padL = 30;
  const padR = 12;
  const padT = 18;
  const chartH = 180;
  const baseY = padT + chartH;
  const width = padL + data.length * (barW + gap) + padR;
  const height = baseY + 40;

  const bars = data
    .map((d, i) => {
      const h = Math.max(2, Math.round((d.value / max) * chartH));
      const x = padL + i * (barW + gap);
      const y = baseY - h;
      const label = d.label.length > 11 ? `${d.label.slice(0, 10)}…` : d.label;
      return `<g>
        <rect x="${x}" y="${y}" width="${barW}" height="${h}" rx="2" fill="#2563eb"/>
        <text x="${x + barW / 2}" y="${y - 6}" class="bar-val" text-anchor="middle">${d.value}</text>
        <text x="${x + barW / 2}" y="${baseY + 18}" class="bar-lbl" text-anchor="middle">${escapeHtml(label)}</text>
      </g>`;
    })
    .join("");

  return `<div class="chart column-chart">
    <svg viewBox="0 0 ${width} ${height}" width="100%" height="${height}" preserveAspectRatio="xMidYMid meet" role="img" aria-label="Findings by category">
      <line x1="${padL - 6}" y1="${baseY}" x2="${width - padR}" y2="${baseY}" stroke="#cbd5e1"/>
      ${bars}
    </svg>
  </div>`;
}

function statBoxes(severity) {
  const box = (label, key, cls) =>
    `<div class="stat-box"><div class="stat-label">${label}</div><div class="stat-value ${cls}">${Number(severity[key] || 0)}</div></div>`;
  return `<section class="stats-grid">
    ${box("Critical", "Critical", "critical")}
    ${box("High", "High", "high")}
    ${box("Medium", "Medium", "medium")}
    ${box("Low", "Low", "low")}
  </section>`;
}

function chartsBlock(stats) {
  const severity = stats?.by_severity || {};
  const categories = stats?.by_category || [];
  return `<section class="charts-grid">
    <div class="chart-card"><h2>Severity distribution</h2>${buildSeverityPie(severity)}</div>
    <div class="chart-card"><h2>Findings by category</h2>${buildCategoryColumns(categories)}</div>
  </section>`;
}

const REPORT_CSS = `
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: Inter, system-ui, sans-serif; background: #f4f6f9; color: #1e293b; padding: 2rem; line-height: 1.6; }
  .wrap { max-width: 980px; margin: 0 auto; background: #fff; border: 1px solid #dce3ed; padding: 2rem; }
  h1 { font-size: 1.5rem; margin-bottom: 0.5rem; border-bottom: 2px solid #2563eb; padding-bottom: 0.5rem; }
  h2 { font-size: 1.1rem; margin: 1.5rem 0 0.75rem; color: #2563eb; }
  h3 { font-size: 1rem; margin: 1rem 0 0.5rem; }
  p { margin-bottom: 0.75rem; }
  ul, ol { margin: 0.5rem 0 0.75rem 1.4rem; }
  li { margin-bottom: 0.3rem; }
  .meta { font-size: 0.8rem; color: #64748b; margin-bottom: 1.25rem; }
  .stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.75rem; margin: 1.25rem 0; }
  .stat-box { border: 1px solid #dce3ed; padding: 1rem; text-align: center; }
  .stat-label { font-size: 0.75rem; color: #64748b; text-transform: uppercase; }
  .stat-value { font-size: 1.75rem; font-weight: 700; margin-top: 0.25rem; }
  .critical { color: #dc2626; } .high { color: #ea580c; } .medium { color: #d97706; } .low { color: #16a34a; }
  .charts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin: 1.25rem 0; }
  .chart-card { border: 1px solid #dce3ed; padding: 1rem; }
  .chart-card h2 { margin-top: 0; }
  .chart { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
  .pie-chart svg { flex: 0 0 auto; }
  .column-chart { display: block; }
  .chart .legend { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.82rem; }
  .legend-item { display: flex; align-items: center; gap: 0.4rem; }
  .legend-swatch { width: 12px; height: 12px; border-radius: 2px; display: inline-block; }
  .legend-pct { color: #64748b; }
  .chart-empty { color: #94a3b8; font-size: 0.85rem; padding: 1.5rem 0; text-align: center; }
  .bar-val { font-size: 11px; fill: #1e293b; font-weight: 600; }
  .bar-lbl { font-size: 10px; fill: #64748b; }
  .risk-banner { display: flex; align-items: center; gap: 1rem; border: 1px solid #dce3ed; border-left-width: 6px; padding: 1rem 1.25rem; margin: 1rem 0 1.25rem; }
  .risk-banner.sev-critical { border-left-color: #dc2626; background: #fef2f2; }
  .risk-banner.sev-high { border-left-color: #ea580c; background: #fff7ed; }
  .risk-banner.sev-medium { border-left-color: #d97706; background: #fffbeb; }
  .risk-banner.sev-low { border-left-color: #16a34a; background: #f0fdf4; }
  .risk-banner.sev-info { border-left-color: #64748b; background: #f8fafc; }
  .risk-level { font-size: 1.35rem; font-weight: 800; text-transform: uppercase; letter-spacing: 0.04em; white-space: nowrap; }
  .risk-banner.sev-critical .risk-level { color: #dc2626; }
  .risk-banner.sev-high .risk-level { color: #ea580c; }
  .risk-banner.sev-medium .risk-level { color: #d97706; }
  .risk-banner.sev-low .risk-level { color: #16a34a; }
  .risk-banner.sev-info .risk-level { color: #64748b; }
  .risk-rationale { font-size: 0.9rem; color: #334155; }
  .data-table { width: 100%; border-collapse: collapse; margin: 0.75rem 0 1.5rem; font-size: 0.9rem; table-layout: fixed; }
  .data-table th, .data-table td { border: 1px solid #dce3ed; padding: 0.5rem 0.75rem; text-align: left; vertical-align: top; word-wrap: break-word; overflow-wrap: anywhere; }
  .data-table th { background: #f8fafc; font-weight: 600; }
  .data-table .num { text-align: right; font-variant-numeric: tabular-nums; }
  .badge { display: inline-block; padding: 0.1rem 0.5rem; border-radius: 3px; font-size: 0.72rem; font-weight: 700; text-transform: uppercase; color: #fff; }
  .badge.sev-critical { background: #dc2626; } .badge.sev-high { background: #ea580c; }
  .badge.sev-medium { background: #d97706; } .badge.sev-low { background: #16a34a; } .badge.sev-info { background: #64748b; }
  .tag { display: inline-block; padding: 0.1rem 0.5rem; border: 1px solid #cbd5e1; border-radius: 3px; font-size: 0.7rem; color: #475569; background: #f8fafc; }
  .finding { border: 1px solid #dce3ed; border-left-width: 5px; padding: 1rem 1.25rem; margin: 1rem 0; }
  .finding.sev-critical { border-left-color: #dc2626; } .finding.sev-high { border-left-color: #ea580c; }
  .finding.sev-medium { border-left-color: #d97706; } .finding.sev-low { border-left-color: #16a34a; } .finding.sev-info { border-left-color: #64748b; }
  .finding-head { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; margin-bottom: 0.5rem; }
  .finding-title { font-size: 1.05rem; font-weight: 700; }
  .finding-loc { font-family: "JetBrains Mono", ui-monospace, monospace; font-size: 0.82rem; color: #475569; margin-bottom: 0.6rem; }
  .finding h4 { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; color: #64748b; margin: 0.75rem 0 0.25rem; }
  .finding pre { background: #0f172a; color: #e2e8f0; padding: 0.75rem; overflow: auto; font-family: "JetBrains Mono", ui-monospace, monospace; font-size: 0.78rem; white-space: pre-wrap; word-break: break-word; }
  .empty-note { color: #94a3b8; font-style: italic; }
  @media print { .charts-grid { grid-template-columns: 1fr 1fr; } .chart-card, .finding { break-inside: avoid; } }
`;

function docShell(title, bodyHtml) {
  return `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"/><meta name="viewport" content="width=device-width, initial-scale=1.0"/><title>${escapeHtml(title)}</title><style>${REPORT_CSS}</style></head>
<body><div class="wrap">${bodyHtml}</div></body></html>`;
}

/* ============================================================
   Template 1 — Executive Report
   High-level summary · risk overview · key findings · recommended actions
   ============================================================ */
function renderExecutive({ projectId, executive, stats }) {
  const exec = executive || {};
  const risk = exec.risk || {};
  const riskLevel = risk.level || "Info";
  const stat = stats || {};

  const keyFindings = Array.isArray(exec.key_findings) ? exec.key_findings : [];
  const findingRows = keyFindings.length
    ? keyFindings
        .map(
          (f) =>
            `<tr><td><span class="badge ${sevClass(f.severity)}">${escapeHtml(f.severity || "Info")}</span></td><td>${escapeHtml(f.title)}</td><td>${escapeHtml(f.area || "—")}</td></tr>`
        )
        .join("")
    : `<tr><td colspan="3" class="empty-note">No key findings reported.</td></tr>`;

  const recommendations = Array.isArray(exec.recommendations) ? exec.recommendations : [];
  const recList = recommendations.length
    ? `<ol>${recommendations.map((r) => `<li>${escapeHtml(r)}</li>`).join("")}</ol>`
    : `<p class="empty-note">No recommendations reported.</p>`;

  const body = `
    <h1>Executive Security Report</h1>
    <div class="meta">Project: ${escapeHtml(projectId)} · Generated by AgentReport</div>

    <h2>Risk overview</h2>
    <div class="risk-banner ${sevClass(riskLevel)}">
      <div class="risk-level">${escapeHtml(riskLevel)}</div>
      <div class="risk-rationale">${escapeHtml(risk.rationale || "Overall risk derived from the combined SAST and DAST findings.")}</div>
    </div>
    ${statBoxes(stat.by_severity || {})}
    ${chartsBlock(stat)}

    <h2>High-level summary</h2>
    <p>${escapeHtml(exec.summary || "No summary generated.")}</p>

    <h2>Key findings</h2>
    <table class="data-table"><thead><tr><th style="width:14%">Severity</th><th>Title</th><th style="width:28%">Affected area</th></tr></thead><tbody>${findingRows}</tbody></table>

    <h2>Recommended actions</h2>
    ${recList}
  `;
  return docShell("Executive Security Report", body);
}

/* ============================================================
   Template 2 — Technical Report
   Detailed findings · evidence & reproduction · technical impact · remediation
   ============================================================ */
function renderTechnical({ projectId, technical, stats }) {
  const tech = technical || {};
  const stat = stats || {};
  const findings = Array.isArray(tech.findings) ? tech.findings : [];

  const findingBlocks = findings.length
    ? findings
        .map((f, i) => {
          const repro = Array.isArray(f.reproduction) ? f.reproduction.filter(Boolean) : [];
          const refs = Array.isArray(f.references) ? f.references.filter(Boolean) : [];
          const reproHtml = repro.length
            ? `<h4>Reproduction steps</h4><ol>${repro.map((s) => `<li>${escapeHtml(s)}</li>`).join("")}</ol>`
            : "";
          const refsHtml = refs.length
            ? `<h4>References</h4><ul>${refs.map((s) => `<li>${escapeHtml(s)}</li>`).join("")}</ul>`
            : "";
          const evidenceHtml = f.evidence
            ? `<h4>Evidence</h4><pre>${escapeHtml(f.evidence)}</pre>`
            : "";
          return `<div class="finding ${sevClass(f.severity)}">
            <div class="finding-head">
              <span class="badge ${sevClass(f.severity)}">${escapeHtml(f.severity || "Info")}</span>
              <span class="finding-title">${i + 1}. ${escapeHtml(f.title || "Untitled finding")}</span>
              <span class="tag">${escapeHtml(f.source || "—")}</span>
            </div>
            <div class="finding-loc">${escapeHtml(f.location || "location not specified")}</div>
            <h4>Description</h4><p>${escapeHtml(f.description || "—")}</p>
            ${evidenceHtml}
            ${reproHtml}
            <h4>Technical impact</h4><p>${escapeHtml(f.impact || "—")}</p>
            <h4>Remediation guidance</h4><p>${escapeHtml(f.remediation || "—")}</p>
            ${refsHtml}
          </div>`;
        })
        .join("")
    : `<p class="empty-note">No detailed findings were reported.</p>`;

  const body = `
    <h1>Technical Security Report</h1>
    <div class="meta">Project: ${escapeHtml(projectId)} · Generated by AgentReport</div>

    <h2>Summary statistics</h2>
    ${statBoxes(stat.by_severity || {})}
    ${chartsBlock(stat)}

    <h2>Detailed findings</h2>
    ${findingBlocks}
  `;
  return docShell("Technical Security Report", body);
}

module.exports = { escapeHtml, renderExecutive, renderTechnical, REPORT_CSS };
