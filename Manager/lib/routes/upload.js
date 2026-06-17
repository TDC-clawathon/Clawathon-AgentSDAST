const express = require("express");
const multer = require("multer");
const { getPool } = require("../db");
const { uploadFile, SOURCE_OBJECT_NAME, swaggerObjectName } = require("../storage/minio");
const { getScanRecord } = require("../services/scan");

const router = express.Router({ mergeParams: true });
const MAX_UPLOAD_BYTES = 200 * 1024 * 1024; // 200 MB
const upload = multer({
  storage: multer.memoryStorage(),
  limits: { fileSize: MAX_UPLOAD_BYTES },
});

// Wrap multer so a too-large file returns a clear 413 JSON message instead of
// a generic 500. Keeps the 200 MB limit's error friendly and machine-readable.
function uploadSingle(field) {
  const mw = upload.single(field);
  return (req, res, next) =>
    mw(req, res, (err) => {
      if (err) {
        if (err.code === "LIMIT_FILE_SIZE") {
          return res
            .status(413)
            .json({ error: "File exceeds the maximum upload size of 200 MB." });
        }
        return res.status(400).json({ error: err.message || "Upload failed" });
      }
      next();
    });
}

async function assertDraftScan(scanId) {
  const record = await getScanRecord(scanId);
  if (!record) {
    const err = new Error("Scan record not found");
    err.status = 404;
    throw err;
  }
  if (record.status !== "draft") {
    const err = new Error("Scan is not in draft status");
    err.status = 400;
    throw err;
  }
  return record;
}

router.post("/:scanId/source", uploadSingle("file"), async (req, res) => {
  try {
    const scanId = req.params.scanId;
    await assertDraftScan(scanId);

    if (!req.file) {
      return res.status(400).json({ error: "No file provided" });
    }
    if (!req.file.originalname.toLowerCase().endsWith(".zip")) {
      return res.status(400).json({ error: "Source file must be a .zip" });
    }

    const uploaded = await uploadFile(req.file, scanId, SOURCE_OBJECT_NAME);
    const pool = getPool();
    await pool.execute("UPDATE ScanRecord SET linksource = ? WHERE id = ?", [
      uploaded.path,
      scanId,
    ]);

    res.json({
      scan_id: scanId,
      path: uploaded.path,
      url: uploaded.url,
      filename: uploaded.filename,
    });
  } catch (err) {
    console.error("Source upload failed:", err);
    res.status(err.status || 500).json({ error: err.message || "Upload failed" });
  }
});

router.post("/:scanId/swagger", uploadSingle("file"), async (req, res) => {
  try {
    const scanId = req.params.scanId;
    await assertDraftScan(scanId);

    if (!req.file) {
      return res.status(400).json({ error: "No file provided" });
    }

    const name = req.file.originalname.toLowerCase();
    const allowed = [".json", ".yaml", ".yml"];
    if (!allowed.some((ext) => name.endsWith(ext))) {
      return res.status(400).json({ error: "Swagger file must be .json, .yaml or .yml" });
    }

    const canonicalName = swaggerObjectName(req.file.originalname);
    const uploaded = await uploadFile(req.file, scanId, canonicalName);
    const pool = getPool();
    await pool.execute("UPDATE ScanRecord SET linkrawswagger = ? WHERE id = ?", [
      uploaded.path,
      scanId,
    ]);

    res.json({
      scan_id: scanId,
      path: uploaded.path,
      url: uploaded.url,
      filename: uploaded.filename,
    });
  } catch (err) {
    console.error("Swagger upload failed:", err);
    res.status(err.status || 500).json({ error: err.message || "Upload failed" });
  }
});

module.exports = router;
