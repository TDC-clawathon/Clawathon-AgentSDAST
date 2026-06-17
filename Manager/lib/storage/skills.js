const {
  S3Client,
  PutObjectCommand,
  GetObjectCommand,
  DeleteObjectCommand,
  ListObjectsV2Command,
} = require("@aws-sdk/client-s3");

const s3 = new S3Client({
  endpoint: process.env.MINIO_ENDPOINT || "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: process.env.MINIO_ROOT_USER,
    secretAccessKey: process.env.MINIO_ROOT_PASSWORD,
  },
  forcePathStyle: true,
});

const {
  normalizeRelativePath,
  assertObjectKeyUnderPrefix,
} = require("../security/path");

const bucket = process.env.MINIO_BUCKET || "agentsdast";
const SKILLS_ROOT = "skills";
const VALID_AGENTS = new Set(["sast", "dast", "report"]);

function assertAgent(agent) {
  const key = String(agent || "").toLowerCase();
  if (!VALID_AGENTS.has(key)) {
    throw new Error("agent must be sast, dast, or report");
  }
  return key;
}

function prefixFor(agent) {
  return `${SKILLS_ROOT}/${assertAgent(agent)}/`;
}

function objectKey(agent, relativePath) {
  const prefix = prefixFor(agent);
  const key = `${prefix}${normalizeRelativePath(relativePath)}`;
  return assertObjectKeyUnderPrefix(key, prefix);
}

function contentTypeFor(path) {
  const lower = path.toLowerCase();
  if (lower.endsWith(".md")) return "text/markdown; charset=utf-8";
  if (lower.endsWith(".yaml") || lower.endsWith(".yml")) return "application/x-yaml; charset=utf-8";
  if (lower.endsWith(".json")) return "application/json; charset=utf-8";
  return "text/plain; charset=utf-8";
}

async function listSkills(agent) {
  const prefix = prefixFor(agent);
  const files = [];
  let token;

  do {
    const res = await s3.send(
      new ListObjectsV2Command({
        Bucket: bucket,
        Prefix: prefix,
        ContinuationToken: token,
      })
    );

    for (const item of res.Contents || []) {
      if (!item.Key || item.Key === prefix || item.Key.endsWith("/")) continue;
      files.push({
        path: item.Key.slice(prefix.length),
        size: item.Size || 0,
        last_modified: item.LastModified ? item.LastModified.toISOString() : null,
      });
    }

    token = res.IsTruncated ? res.NextContinuationToken : undefined;
  } while (token);

  files.sort((a, b) => a.path.localeCompare(b.path));
  return files;
}

async function getSkill(agent, relativePath) {
  const path = normalizeRelativePath(relativePath);
  const res = await s3.send(
    new GetObjectCommand({
      Bucket: bucket,
      Key: objectKey(agent, path),
    })
  );

  const content = await res.Body.transformToString("utf-8");
  return {
    agent: assertAgent(agent),
    path,
    object_key: objectKey(agent, path),
    content,
    content_type: res.ContentType || contentTypeFor(path),
    last_modified: res.LastModified ? res.LastModified.toISOString() : null,
  };
}

async function putSkill(agent, relativePath, content) {
  const path = normalizeRelativePath(relativePath);
  const body = typeof content === "string" ? content : String(content ?? "");

  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: objectKey(agent, path),
      Body: body,
      ContentType: contentTypeFor(path),
    })
  );

  return {
    agent: assertAgent(agent),
    path,
    object_key: objectKey(agent, path),
    size: Buffer.byteLength(body, "utf-8"),
  };
}

async function deleteSkill(agent, relativePath) {
  const path = normalizeRelativePath(relativePath);
  await s3.send(
    new DeleteObjectCommand({
      Bucket: bucket,
      Key: objectKey(agent, path),
    })
  );

  return {
    agent: assertAgent(agent),
    path,
    object_key: objectKey(agent, path),
  };
}

module.exports = {
  listSkills,
  getSkill,
  putSkill,
  deleteSkill,
  prefixFor,
};
