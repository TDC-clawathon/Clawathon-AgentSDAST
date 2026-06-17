const fs = require("fs/promises");
const path = require("path");

const DEFAULT_PROMPT =
  "Write executive and technical security reports from SAST and DAST inputs.";

// Read every markdown file in a skill folder (SKILL.md first, then the rest
// alphabetically) and concatenate them. This replaces hardcoded single-file
// registration with automatic discovery of whatever the folder contains.
async function readSkillFolder(dir) {
  let entries;
  try {
    entries = await fs.readdir(dir, { withFileTypes: true });
  } catch {
    return [];
  }

  const mdFiles = entries
    .filter((e) => e.isFile() && e.name.toLowerCase().endsWith(".md"))
    .map((e) => e.name)
    .sort((a, b) => {
      if (a === "SKILL.md") return -1;
      if (b === "SKILL.md") return 1;
      return a.localeCompare(b);
    });

  const parts = [];
  for (const name of mdFiles) {
    try {
      const content = await fs.readFile(path.join(dir, name), "utf-8");
      if (content.trim()) parts.push(`<!-- ${name} -->\n${content.trim()}`);
    } catch {
      /* skip unreadable file */
    }
  }
  return parts;
}

// Load the report skill prompt by auto-discovering skill files from the
// report folder and any shared folder (skills/shared/*.md).
async function loadSkillPrompt() {
  const reportDir = process.env.SKILLS_REPORT_DIR || "/app/skills/report";
  const sharedDir =
    process.env.SKILLS_SHARED_DIR || path.join(path.dirname(reportDir), "shared");

  const [sharedParts, reportParts] = await Promise.all([
    readSkillFolder(sharedDir),
    readSkillFolder(reportDir),
  ]);

  const combined = [...sharedParts, ...reportParts].join("\n\n");
  return combined.trim() || DEFAULT_PROMPT;
}

module.exports = { loadSkillPrompt, readSkillFolder };
