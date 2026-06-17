// HTTP Basic Auth for the admin portal (/adm) and admin APIs.
//
// Credentials come from the environment (ADMIN_USERNAME / ADMIN_PASSWORD),
// generated at setup via `openssl rand -hex 12` and stored in .env. If
// ADMIN_PASSWORD is missing, a random one is generated for the process (logged
// once) so the admin surface is never left unprotected with a hardcoded default.
const crypto = require("crypto");

const REALM = "AgentSDAST Admin";
let runtimePassword = null;

function adminUsername() {
  return process.env.ADMIN_USERNAME || "admin";
}

function adminPassword() {
  if (process.env.ADMIN_PASSWORD) return process.env.ADMIN_PASSWORD;
  if (!runtimePassword) {
    runtimePassword = crypto.randomBytes(12).toString("hex");
    console.warn(
      `[auth] ADMIN_PASSWORD not set — generated a temporary admin password for this run: ${runtimePassword}`
    );
  }
  return runtimePassword;
}

function timingEqual(a, b) {
  const ab = Buffer.from(String(a));
  const bb = Buffer.from(String(b));
  if (ab.length !== bb.length) return false;
  return crypto.timingSafeEqual(ab, bb);
}

function parseBasic(req) {
  const header = req.headers.authorization || "";
  const m = /^Basic\s+(.+)$/i.exec(header);
  if (!m) return null;
  let decoded;
  try {
    decoded = Buffer.from(m[1], "base64").toString("utf8");
  } catch {
    return null;
  }
  const i = decoded.indexOf(":");
  if (i < 0) return null;
  return { user: decoded.slice(0, i), pass: decoded.slice(i + 1) };
}

// isAdmin reports whether the request carries valid admin Basic credentials.
function isAdmin(req) {
  const creds = parseBasic(req);
  if (!creds) return false;
  return timingEqual(creds.user, adminUsername()) && timingEqual(creds.pass, adminPassword());
}

function challenge(res) {
  res.set("WWW-Authenticate", `Basic realm="${REALM}", charset="UTF-8"`);
}

// requireAdmin gates admin routes/APIs. Backend enforcement — never relies on
// the UI. 401 for missing/invalid credentials (browser re-prompts); 403 when a
// different, non-admin principal authenticates.
function requireAdmin(req, res, next) {
  const creds = parseBasic(req);
  if (!creds) {
    challenge(res);
    return res.status(401).json({ error: "Authentication required" });
  }
  const userOk = timingEqual(creds.user, adminUsername());
  const passOk = timingEqual(creds.pass, adminPassword());
  if (userOk && passOk) return next();
  if (userOk) {
    // Correct admin user, wrong password → re-prompt.
    challenge(res);
    return res.status(401).json({ error: "Invalid credentials" });
  }
  // Authenticated as some other principal → forbidden (insufficient privileges).
  return res.status(403).json({ error: "Forbidden — admin privileges required" });
}

module.exports = { requireAdmin, isAdmin, REALM };
