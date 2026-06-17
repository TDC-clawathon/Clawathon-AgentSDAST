const ALLOWED_EXTENSIONS = new Set([".md", ".yaml", ".yml", ".json"]);
const MAX_RELATIVE_PATH_LENGTH = 512;
const MAX_SEGMENTS = 32;
const SEGMENT_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

function decodePathIteratively(input, maxPasses = 3) {
  let value = input;
  for (let i = 0; i < maxPasses; i++) {
    if (!value.includes("%")) {
      break;
    }
    try {
      const next = decodeURIComponent(value.replace(/\+/g, " "));
      if (next === value) {
        break;
      }
      value = next;
    } catch {
      throw new Error("Invalid skill path encoding");
    }
  }
  return value;
}

function normalizeRelativePath(relativePath) {
  let raw = String(relativePath ?? "");
  if (/[\0\r\n]/.test(raw)) {
    throw new Error("Invalid skill path");
  }

  raw = decodePathIteratively(raw.trim());
  raw = raw.replace(/\\/g, "/").replace(/^\/+/, "");

  if (!raw || raw.includes("..") || raw.includes("//")) {
    throw new Error("Invalid skill path");
  }

  const segments = raw.split("/").filter(Boolean);
  if (!segments.length) {
    throw new Error("Invalid skill path");
  }
  if (segments.length > MAX_SEGMENTS) {
    throw new Error("Skill path is too deep");
  }

  for (const segment of segments) {
    if (segment === "." || segment === "..") {
      throw new Error("Invalid skill path");
    }
    if (!SEGMENT_RE.test(segment)) {
      throw new Error("Skill path contains invalid characters");
    }
  }

  const normalized = segments.join("/");
  if (normalized.length > MAX_RELATIVE_PATH_LENGTH) {
    throw new Error("Skill path is too long");
  }

  const lower = normalized.toLowerCase();
  const dot = lower.lastIndexOf(".");
  const ext = dot >= 0 ? lower.slice(dot) : "";
  if (!ALLOWED_EXTENSIONS.has(ext)) {
    throw new Error("Skill file extension is not allowed");
  }

  return normalized;
}

function assertObjectKeyUnderPrefix(objectKey, prefix) {
  if (!objectKey.startsWith(prefix)) {
    throw new Error("Invalid skill path");
  }
  if (objectKey.includes("..") || objectKey.includes("//")) {
    throw new Error("Invalid skill path");
  }

  const remainder = objectKey.slice(prefix.length);
  if (!remainder || remainder.startsWith("/")) {
    throw new Error("Invalid skill path");
  }

  return objectKey;
}

module.exports = {
  normalizeRelativePath,
  assertObjectKeyUnderPrefix,
};
