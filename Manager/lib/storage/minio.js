const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const path = require("path");

const s3 = new S3Client({
  endpoint: process.env.MINIO_ENDPOINT || "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: process.env.MINIO_ROOT_USER,
    secretAccessKey: process.env.MINIO_ROOT_PASSWORD,
  },
  forcePathStyle: true,
});

const bucket = process.env.MINIO_BUCKET || "agentsdast";
const publicBase =
  process.env.MINIO_PUBLIC_URL ||
  `http://localhost:${process.env.MINIO_API_PORT || 9000}`;

function safeFilename(originalname) {
  const base = path.basename(originalname || "upload");
  if (!base || base === "." || base === ".." || base.includes("..")) {
    throw new Error("Invalid filename");
  }
  return base;
}

function toRelativePath(filename) {
  return `/raw/${filename}`;
}

function toObjectKey(scanId, filename) {
  return `${scanId}/raw/${filename}`;
}

function toPublicUrl(objectKey) {
  return `${publicBase}/${bucket}/${objectKey}`;
}

/** Resolve stored path (/raw/file.zip or legacy full URL) to a MinIO object key. */
function resolveObjectKey(storedPath, scanId) {
  if (!storedPath) return "";

  const trimmed = String(storedPath).trim();
  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
    try {
      const parts = new URL(trimmed).pathname.split("/").filter(Boolean);
      if (parts.length >= 2) return parts.slice(1).join("/");
      return parts.join("/");
    } catch {
      return "";
    }
  }

  const rel = trimmed.replace(/^\/+/, "");
  if (!rel) return "";

  if (rel.startsWith("raw/")) {
    return scanId ? `${scanId}/${rel}` : rel;
  }

  if (scanId && !rel.startsWith(`${scanId}/`)) {
    return `${scanId}/${rel}`;
  }

  return rel;
}

const SOURCE_OBJECT_NAME = "source_code.zip";

function swaggerObjectName(originalname) {
  const name = String(originalname || "").toLowerCase();
  if (name.endsWith(".json")) return "raw_swagger.json";
  if (name.endsWith(".yaml")) return "raw_swagger.yaml";
  if (name.endsWith(".yml")) return "raw_swagger.yml";
  throw new Error("Swagger file must be .json, .yaml or .yml");
}

async function uploadFile(file, scanId, targetFilename) {
  const filename = safeFilename(targetFilename || file.originalname);
  const objectKey = toObjectKey(scanId, filename);

  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: objectKey,
      Body: file.buffer,
      ContentType: file.mimetype || "application/octet-stream",
    })
  );

  const relativePath = toRelativePath(filename);
  return {
    path: relativePath,
    object_key: objectKey,
    url: toPublicUrl(objectKey),
    filename,
  };
}

module.exports = {
  uploadFile,
  resolveObjectKey,
  toRelativePath,
  toObjectKey,
  toPublicUrl,
  SOURCE_OBJECT_NAME,
  swaggerObjectName,
};
