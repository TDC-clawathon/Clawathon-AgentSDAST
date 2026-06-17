function getConfigHealth() {
  const apiBase = process.env.LLM_API_BASE || "";
  const apiKey = process.env.LLM_API_KEY || "";

  if (!apiBase || !apiKey) {
    return {
      status: "down",
      message: !apiBase ? "LLM_API_BASE not set" : "LLM_API_KEY not set",
      api_base: apiBase || null,
      connected: false,
    };
  }

  return {
    status: "up",
    message: apiBase,
    api_base: apiBase,
    connected: null,
  };
}

async function checkConnection() {
  const base = getConfigHealth();
  if (base.status !== "up") {
    return { ...base, connected: false };
  }

  try {
    const models = await fetchRemoteModels();
    return {
      ...base,
      connected: true,
      models_count: models.length,
      message: `Connected · ${models.length} models`,
    };
  } catch (err) {
    return {
      ...base,
      status: "down",
      connected: false,
      message: err.message || "LLM API unreachable",
    };
  }
}

function isModelEnabled(model) {
  if (model.status === undefined || model.status === null || model.status === "") {
    return true;
  }
  return String(model.status).toLowerCase() === "enabled";
}

async function fetchRemoteModels() {
  const apiBase = (process.env.LLM_API_BASE || "").replace(/\/$/, "");
  const apiKey = process.env.LLM_API_KEY || "";
  if (!apiBase || !apiKey) {
    return [];
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15000);

  try {
    const res = await fetch(`${apiBase}/models`, {
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
    });

    const text = await res.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      throw new Error("Invalid JSON from LLM /models");
    }

    if (!res.ok) {
      throw new Error(data?.error?.message || data?.error || `LLM HTTP ${res.status}`);
    }

    const raw = Array.isArray(data?.data) ? data.data : [];
    return raw
      .filter(isModelEnabled)
      .map((m) => ({
        id: m.id,
        owned_by: m.owned_by || null,
        status: m.status || null,
      }))
      .filter((m) => m.id);
  } finally {
    clearTimeout(timer);
  }
}

function getDefaultModels() {
  return {
    sast: process.env.AGENT_SAST_MODEL || "gpt-4o",
    dast: process.env.AGENT_DAST_MODEL || "gpt-4o-mini",
    report: process.env.AGENT_REPORT_MODEL || "gpt-4o",
  };
}

function listAvailableModels(remoteModels = []) {
  const remoteIds = remoteModels.map((m) => m.id);
  const defaults = getDefaultModels();
  const fromEnv = (process.env.AVAILABLE_LLM_MODELS || "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  return [
    ...new Set([
      ...remoteIds,
      ...fromEnv,
      defaults.sast,
      defaults.dast,
      defaults.report,
    ].filter(Boolean)),
  ];
}

let responsesModelCache = { at: 0, models: [] };
const RESPONSES_CACHE_MS = 10 * 60 * 1000;

async function probeResponsesModel(modelId) {
  const apiBase = (process.env.LLM_API_BASE || "").replace(/\/$/, "");
  const apiKey = process.env.LLM_API_KEY || "";
  if (!apiBase || !apiKey || !modelId) return false;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 45000);
  try {
    const res = await fetch(`${apiBase}/responses`, {
      method: "POST",
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify({
        model: modelId,
        input: "ping",
        max_output_tokens: 8,
      }),
    });
    return res.ok;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

async function fetchResponsesCompatibleModels(modelIds = []) {
  const now = Date.now();
  if (responsesModelCache.models.length && now - responsesModelCache.at < RESPONSES_CACHE_MS) {
    return responsesModelCache.models;
  }

  const compatible = [];
  for (const modelId of modelIds) {
    if (await probeResponsesModel(modelId)) {
      compatible.push(modelId);
    }
  }

  responsesModelCache = { at: now, models: compatible };
  return compatible;
}

// probeChatModel verifies a model is actually usable via a minimal
// chat-completion (catches "model not found"/404 and non-chat models that the
// /models listing still advertises).
async function probeChatModel(modelId) {
  const apiBase = (process.env.LLM_API_BASE || "").replace(/\/$/, "");
  const apiKey = process.env.LLM_API_KEY || "";
  if (!apiBase || !apiKey || !modelId) return false;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 20000);
  try {
    const res = await fetch(`${apiBase}/chat/completions`, {
      method: "POST",
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify({
        model: modelId,
        messages: [{ role: "user", content: "ping" }],
        max_tokens: 1,
      }),
    });
    if (res.ok) return true;
    // Exclude ONLY when the model is definitively unavailable. Rate limits (429),
    // server (5xx) or auth errors are not the model's fault — keep it rather than
    // hide a usable model.
    if (res.status === 404) return false;
    if (res.status === 400 || res.status === 422) {
      const body = (await res.text().catch(() => "")).toLowerCase();
      if (/not found|not exist|unknown model|no such model|invalid model/.test(body)) {
        return false;
      }
    }
    return true;
  } catch {
    // Network/timeout is transient — don't over-filter.
    return true;
  } finally {
    clearTimeout(timer);
  }
}

let usableModelCache = { at: 0, models: [] };
const USABLE_CACHE_MS = 10 * 60 * 1000;

// fetchUsableModels returns only the models that pass a live chat-completion
// probe, cached for 10 min to avoid hammering the provider. An empty result is
// not cached so a transient provider outage re-probes on the next call.
async function fetchUsableModels(modelIds = []) {
  const now = Date.now();
  if (usableModelCache.models.length && now - usableModelCache.at < USABLE_CACHE_MS) {
    return usableModelCache.models;
  }
  const ids = [...new Set(modelIds.filter(Boolean))];
  const usable = [];
  const CONCURRENCY = 6; // probe in small batches to respect provider rate limits
  for (let i = 0; i < ids.length; i += CONCURRENCY) {
    const batch = ids.slice(i, i + CONCURRENCY);
    const results = await Promise.all(
      batch.map(async (id) => ({ id, ok: await probeChatModel(id) }))
    );
    for (const r of results) if (r.ok) usable.push(r.id);
  }
  if (usable.length) usableModelCache = { at: now, models: usable };
  return usable;
}

module.exports = {
  getHealth: getConfigHealth,
  checkConnection,
  fetchRemoteModels,
  getDefaultModels,
  listAvailableModels,
  fetchResponsesCompatibleModels,
  probeResponsesModel,
  probeChatModel,
  fetchUsableModels,
};
