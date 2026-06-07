function joinUrl(baseUrl, path) {
  return `${String(baseUrl || "").replace(/\/+$/, "")}/${String(path || "").replace(/^\/+/, "")}`;
}

function authHeaders(token) {
  const value = String(token || "").trim();
  return value ? { Authorization: `Bearer ${value}` } : {};
}

async function requestJson(baseUrl, path, options = {}) {
  let response;
  try {
    response = await fetch(joinUrl(baseUrl, path), {
      ...options,
      headers: {
        ...(options.headers || {})
      }
    });
  } catch (error) {
    return { ok: false, status: 0, data: null, error };
  }
  const text = await response.text();
  let data = null;
  if (text.trim()) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  return { ok: response.ok, status: response.status, data };
}

function buildQuery(values = {}) {
  const params = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value != null && String(value).trim() !== "") {
      params.set(key, String(value));
    }
  });
  return params.toString();
}

// TMovie is the self-hosted torrent resolver (debrid replacement). It exposes a
// REST API at {serverUrl}/api/v1. See backend internal/handlers/torrent.go.
export const TMovieApi = {

  // Confirm the server is installed/reachable and whether it requires a token.
  async getStatus(serverUrl, token) {
    const response = await requestJson(serverUrl, "api/v1/setup/status", {
      method: "GET",
      headers: authHeaders(token)
    });
    if (!response.ok || !response.data || typeof response.data !== "object") {
      return { ok: false, installed: false };
    }
    return {
      ok: true,
      installed: response.data.installed === true,
      tokenRequired: response.data.tokenRequired === true,
      torrent: response.data.torrent === true,
      version: String(response.data.version || "")
    };
  },

  // Resolve a torrent (magnet or infohash) into a directly playable HTTP URL.
  // The server streams the file while downloading (range/seek supported).
  async resolveTorrent({ serverUrl, token, infoHash, magnetUri, fileIdx, season, episode } = {}) {
    const query = buildQuery({
      magnet: magnetUri || "",
      ih: magnetUri ? "" : infoHash || "",
      f: fileIdx == null ? "" : fileIdx,
      season: season || "",
      episode: episode || ""
    });
    const response = await requestJson(serverUrl, `api/v1/torrent/resolve?${query}`, {
      method: "GET",
      headers: authHeaders(token)
    });
    if (!response.ok || !response.data || typeof response.data !== "object" || !response.data.playUrl) {
      return { status: "failed", url: null };
    }
    return {
      status: "success",
      url: String(response.data.playUrl),
      fileIdx: response.data.fileIdx,
      filename: String(response.data.filename || "")
    };
  },

  // Drop a torrent and delete its cached data on the server (frees disk).
  async stopTorrent({ serverUrl, token, infoHash } = {}) {
    const hash = String(infoHash || "").trim();
    if (!serverUrl || !hash) {
      return false;
    }
    const response = await requestJson(serverUrl, `api/v1/torrent/stop?ih=${encodeURIComponent(hash)}`, {
      method: "POST",
      headers: authHeaders(token)
    });
    return response.ok;
  }

};
