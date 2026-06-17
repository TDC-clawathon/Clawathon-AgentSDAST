const { getPool } = require("../db");
const { S3Client, HeadBucketCommand } = require("@aws-sdk/client-s3");

function mysqlConfig() {
  return {
    host: process.env.MYSQL_HOST || "localhost",
    port: String(process.env.MYSQL_PORT || 3306),
    database: process.env.MYSQL_DATABASE || "",
  };
}

function minioClient() {
  return new S3Client({
    endpoint: process.env.MINIO_ENDPOINT || "http://localhost:9000",
    region: "us-east-1",
    credentials: {
      accessKeyId: process.env.MINIO_ROOT_USER,
      secretAccessKey: process.env.MINIO_ROOT_PASSWORD,
    },
    forcePathStyle: true,
  });
}

async function checkMySQL() {
  const cfg = mysqlConfig();
  try {
    const pool = getPool();
    await pool.query("SELECT 1");
    return {
      status: "up",
      reachable: true,
      ...cfg,
      message: `Connected · ${cfg.host}:${cfg.port}/${cfg.database}`,
    };
  } catch (err) {
    return {
      status: "down",
      reachable: false,
      ...cfg,
      message: err.message || "MySQL unreachable",
    };
  }
}

async function checkMinIO() {
  const endpoint = process.env.MINIO_ENDPOINT || "http://localhost:9000";
  const bucket = process.env.MINIO_BUCKET || "agentsdast";
  const s3 = minioClient();

  try {
    await s3.send(new HeadBucketCommand({ Bucket: bucket }));
    return {
      status: "up",
      reachable: true,
      endpoint,
      bucket,
      message: `Connected · ${bucket}`,
    };
  } catch (err) {
    const code = err?.name || err?.Code;
    const http = err?.$metadata?.httpStatusCode;
    if (code === "NotFound" || http === 404) {
      return {
        status: "degraded",
        reachable: true,
        endpoint,
        bucket,
        message: `Reachable but bucket "${bucket}" not found`,
      };
    }
    return {
      status: "down",
      reachable: false,
      endpoint,
      bucket,
      message: err.message || "MinIO unreachable",
    };
  }
}

async function checkAll() {
  const [mysql, minio] = await Promise.all([checkMySQL(), checkMinIO()]);
  return { mysql, minio };
}

module.exports = {
  checkMySQL,
  checkMinIO,
  checkAll,
};
