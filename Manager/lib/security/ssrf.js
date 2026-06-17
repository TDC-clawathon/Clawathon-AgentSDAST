const dns = require("dns").promises;
const net = require("net");

const BLOCKED_METADATA = new Set([
  "169.254.169.254",
  "169.254.170.2",
  "fd00:ec2::254",
]);

const DEFAULT_ALLOWED_HOSTS = [
  "demo-project",
  "agentsdast-demo-project",
  "localhost",
  "127.0.0.1",
  "::1",
  "host.docker.internal","192.168.1.221"
];

function parseAllowedHosts() {
  const fromEnv = (process.env.API_CHECK_ALLOWED_HOSTS || "")
    .split(",")
    .map((host) => host.trim().toLowerCase())
    .filter(Boolean);
  return new Set([...DEFAULT_ALLOWED_HOSTS, ...fromEnv]);
}

function normalizeHostname(hostname) {
  return String(hostname || "")
    .trim()
    .toLowerCase()
    .replace(/^\[/, "")
    .replace(/\]$/, "");
}

function isBlockedMetadataAddress(address) {
  return BLOCKED_METADATA.has(address.toLowerCase());
}

function isPrivateOrLocalAddress(address) {
  if (isBlockedMetadataAddress(address)) {
    return true;
  }

  if (net.isIPv4(address)) {
    const [a, b] = address.split(".").map(Number);
    if (a === 127) return true;
    if (a === 10) return true;
    if (a === 172 && b >= 16 && b <= 31) return true;
    if (a === 192 && b === 168) return true;
    if (a === 169 && b === 254) return true;
    if (a === 0) return true;
    if (a === 100 && b >= 64 && b <= 127) return true;
    return false;
  }

  if (net.isIPv6(address)) {
    const lower = address.toLowerCase();
    if (lower === "::1" || lower === "::") return true;
    if (lower.startsWith("fc") || lower.startsWith("fd")) return true;
    if (lower.startsWith("fe80")) return true;
    return false;
  }

  return false;
}

function assertHostnameShape(hostname) {
  if (/^\d+$/.test(hostname) && !net.isIPv4(hostname)) {
    throw new Error("URL target is not allowed");
  }
  if (hostname.endsWith(".local") || hostname.endsWith(".internal")) {
    throw new Error("URL target is not allowed");
  }
}

async function resolveHostname(hostname) {
  try {
    return await dns.lookup(hostname, { all: true, verbatim: true });
  } catch {
    throw new Error(`Cannot resolve hostname: ${hostname}`);
  }
}

async function assertAddressAllowed(address, hostAllowlisted) {
  if (isBlockedMetadataAddress(address)) {
    throw new Error("URL target is not allowed");
  }
  if (isPrivateOrLocalAddress(address) && !hostAllowlisted) {
    throw new Error("URL target resolves to a private network address");
  }
}

async function assertSafeHttpUrl(urlString) {
  let parsed;
  try {
    parsed = new URL(urlString);
  } catch {
    throw new Error("Invalid URL");
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("URL must use http or https");
  }

  if (parsed.username || parsed.password) {
    throw new Error("URL credentials are not allowed");
  }

  const hostname = normalizeHostname(parsed.hostname);
  assertHostnameShape(hostname);

  const allowedHosts = parseAllowedHosts();
  const hostAllowlisted = allowedHosts.has(hostname);

  if (net.isIP(hostname)) {
    await assertAddressAllowed(hostname, hostAllowlisted);
    return parsed;
  }

  const records = await resolveHostname(hostname);
  if (!records.length) {
    throw new Error(`Cannot resolve hostname: ${hostname}`);
  }

  for (const { address } of records) {
    await assertAddressAllowed(address, hostAllowlisted);
  }

  return parsed;
}

async function safeFetch(urlString, options = {}, maxRedirects = 3) {
  let current = urlString;

  for (let hop = 0; hop <= maxRedirects; hop++) {
    await assertSafeHttpUrl(current);

    const response = await fetch(current, {
      ...options,
      redirect: "manual",
    });

    if (response.status >= 300 && response.status < 400) {
      const location = response.headers.get("location");
      if (!location) {
        return response;
      }
      if (hop === maxRedirects) {
        throw new Error("Too many redirects");
      }
      current = new URL(location, current).href;
      continue;
    }

    return response;
  }

  throw new Error("Too many redirects");
}

module.exports = {
  assertSafeHttpUrl,
  safeFetch,
};
