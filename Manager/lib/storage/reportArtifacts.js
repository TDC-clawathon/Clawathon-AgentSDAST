const { S3Client, GetObjectCommand, HeadObjectCommand } = require("@aws-sdk/client-s3");

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

async function getObjectText(key) {
  try {
    const res = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
    return await res.Body.transformToString("utf-8");
  } catch {
    return null;
  }
}

async function getObjectBuffer(key) {
  try {
    const res = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
    const bytes = await res.Body.transformToByteArray();
    return { buffer: Buffer.from(bytes), contentType: res.ContentType || null };
  } catch {
    return null;
  }
}

// objectExists reports whether an object is present (cheap HEAD, no body).
async function objectExists(key) {
  try {
    await s3.send(new HeadObjectCommand({ Bucket: bucket, Key: key }));
    return true;
  } catch {
    return false;
  }
}

// reportPdfsReady is the gate for the Report button: BOTH the Executive
// (highlevel.pdf) and Technical (detail.pdf) PDFs must exist in MinIO.
async function reportPdfsReady(scanId) {
  const prefix = `${scanId}/report`;
  const [exec, tech] = await Promise.all([
    objectExists(`${prefix}/highlevel.pdf`),
    objectExists(`${prefix}/detail.pdf`),
  ]);
  return { executive_pdf: exec, technical_pdf: tech, ready: exec && tech };
}

// Reports are HTML-only. highlevel.html = Executive report, detail.html = Technical report.
async function loadScanReports(scanId) {
  const prefix = `${scanId}/report`;
  const [highlevel_html, detail_html, stats_json] = await Promise.all([
    getObjectText(`${prefix}/highlevel.html`),
    getObjectText(`${prefix}/detail.html`),
    getObjectText(`${prefix}/stats.json`),
  ]);

  let stats = null;
  if (stats_json) {
    try {
      stats = JSON.parse(stats_json);
    } catch {
      stats = null;
    }
  }

  return {
    prefix,
    highlevel_html,
    detail_html,
    stats,
    ready: Boolean(highlevel_html || detail_html),
  };
}

module.exports = { loadScanReports, getObjectBuffer, reportPdfsReady, objectExists };
