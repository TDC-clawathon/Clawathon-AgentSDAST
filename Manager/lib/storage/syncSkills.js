const fs = require("fs/promises");
const path = require("path");
const skills = require("./skills");

const SKILL_EXTENSIONS = new Set([".md", ".yaml", ".yml", ".json"]);
const AGENTS = ["sast", "dast", "report"];

async function pathExists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function walkSkillFiles(rootDir) {
  const files = [];

  async function walk(dir, relativeBase = "") {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      const rel = relativeBase ? `${relativeBase}/${entry.name}` : entry.name;
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(fullPath, rel);
        continue;
      }
      if (!entry.isFile()) {
        continue;
      }
      const ext = path.extname(entry.name).toLowerCase();
      if (!SKILL_EXTENSIONS.has(ext)) {
        continue;
      }
      files.push({
        relativePath: rel.replace(/\\/g, "/"),
        fullPath,
      });
    }
  }

  await walk(rootDir);
  files.sort((a, b) => a.relativePath.localeCompare(b.relativePath));
  return files;
}

async function resolveSeedDir(agent) {
  const envKey =
    agent === "sast"
      ? "SKILLS_SEED_SAST_DIR"
      : agent === "dast"
        ? "SKILLS_SEED_DAST_DIR"
        : "SKILLS_SEED_REPORT_DIR";
  if (process.env[envKey]) {
    return process.env[envKey];
  }

  const repoRoot = path.join(__dirname, "..", "..", "..");
  const rootSkills = path.join(repoRoot, "skills", agent);
  if (await pathExists(rootSkills)) {
    return rootSkills;
  }

  const bundled = path.join(__dirname, "..", "..", "skills-seed", agent);
  if (await pathExists(bundled)) {
    return bundled;
  }

  return rootSkills;
}

async function syncAgentSkills(agent, options = {}) {
  const overwrite = options.overwrite === true;
  const sourceDir = options.sourceDir || (await resolveSeedDir(agent));
  const results = {
    agent,
    source_dir: sourceDir,
    uploaded: [],
    skipped: [],
    errors: [],
    missing: false,
  };

  if (!(await pathExists(sourceDir))) {
    results.missing = true;
    return results;
  }

  let existing = new Set();
  if (!overwrite) {
    const listed = await skills.listSkills(agent);
    existing = new Set(listed.map((file) => file.path));
  }

  const files = await walkSkillFiles(sourceDir);
  for (const file of files) {
    try {
      if (!overwrite && existing.has(file.relativePath)) {
        results.skipped.push(file.relativePath);
        continue;
      }
      const content = await fs.readFile(file.fullPath, "utf-8");
      await skills.putSkill(agent, file.relativePath, content);
      results.uploaded.push(file.relativePath);
    } catch (err) {
      results.errors.push({
        path: file.relativePath,
        error: err.message || String(err),
      });
    }
  }

  return results;
}

async function syncAllSeededSkills(options = {}) {
  const summary = {};
  for (const agent of AGENTS) {
    summary[agent] = await syncAgentSkills(agent, options);
  }
  return summary;
}

function isSyncOnStartupEnabled() {
  const value = String(process.env.SKILLS_SYNC_ON_STARTUP ?? "1").toLowerCase();
  return !["0", "false", "no", "off"].includes(value);
}

function isOverwriteByDefault() {
  const value = String(process.env.SKILLS_SYNC_OVERWRITE ?? "0").toLowerCase();
  return ["1", "true", "yes", "on"].includes(value);
}

async function syncOnStartup() {
  if (!isSyncOnStartupEnabled()) {
    console.log("Skills seed sync disabled (SKILLS_SYNC_ON_STARTUP=0)");
    return null;
  }

  const summary = await syncAllSeededSkills({ overwrite: isOverwriteByDefault() });
  for (const agent of AGENTS) {
    const result = summary[agent];
    if (result.missing) {
      console.warn(`Skills seed source missing for ${agent}: ${result.source_dir}`);
      continue;
    }
    console.log(
      `Skills sync (${agent}): uploaded=${result.uploaded.length}, skipped=${result.skipped.length}, errors=${result.errors.length}`
    );
    if (result.errors.length) {
      console.warn(`Skills sync errors (${agent}):`, result.errors);
    }
  }
  return summary;
}

module.exports = {
  syncAgentSkills,
  syncAllSeededSkills,
  syncOnStartup,
  resolveSeedDir,
};
