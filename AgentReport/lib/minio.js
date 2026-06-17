const {
  S3Client,
  GetObjectCommand,
  PutObjectCommand,
} = require("@aws-sdk/client-s3");

const s3 = new S3Client({
  endpoint: process.env.MINIO_ENDPOINT?.startsWith("http")
    ? process.env.MINIO_ENDPOINT
    : `http://${process.env.MINIO_ENDPOINT || "minio:9000"}`,
  region: "us-east-1",
  credentials: {
    accessKeyId: process.env.MINIO_ROOT_USER,
    secretAccessKey: process.env.MINIO_ROOT_PASSWORD,
  },
  forcePathStyle: true,
});

const bucket = process.env.MINIO_BUCKET || "agentsdast";

async function getText(key) {
  try {
    const res = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
    return await res.Body.transformToString("utf-8");
  } catch (err) {
    if (err.name === "NoSuchKey" || err.$metadata?.httpStatusCode === 404) {
      return null;
    }
    throw err;
  }
}

async function putText(key, body, contentType = "text/plain; charset=utf-8") {
  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: key,
      Body: body,
      ContentType: contentType,
    })
  );
}

async function putBuffer(key, body, contentType = "application/octet-stream") {
  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: key,
      Body: body,
      ContentType: contentType,
    })
  );
}

module.exports = { getText, putText, putBuffer, bucket };
