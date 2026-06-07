import { TMovieSettingsStore } from "../../data/local/tmovieSettingsStore.js";
import { TMovieApi } from "../../data/remote/api/tmovieApi.js";

// TMovieResolver turns a torrent stream (infoHash / magnet, no direct URL) into a
// playable HTTP URL via the self-hosted TMovie server, which streams the file
// while it downloads. It mirrors the DirectDebridResolver contract so streamScreen
// can use either resolver interchangeably.

function isMagnetLink(value) {
  return String(value || "").trim().toLowerCase().startsWith("magnet:");
}

function getStreamUrl(stream = {}) {
  return [stream.url, stream.externalUrl].find((value) => value && !isMagnetLink(value)) || null;
}

function torrentMagnetUri(stream = {}) {
  return [stream.url, stream.externalUrl].find((value) => isMagnetLink(value)) || null;
}

function trackerUrl(source) {
  const value = String(source || "").trim();
  if (!value || value.toLowerCase().startsWith("dht:")) {
    return null;
  }
  return value.replace(/^tracker:/i, "").trim() || null;
}

function buildMagnetUri(stream = {}) {
  const existing = torrentMagnetUri(stream);
  if (existing) {
    return existing;
  }
  const hash = String(stream.infoHash || "").trim();
  if (!hash) {
    return null;
  }
  const displayName = String(stream.title || stream.name || "").trim();
  const trackers = (Array.isArray(stream.sources) ? stream.sources : [])
    .map(trackerUrl)
    .filter(Boolean)
    .filter((value, index, list) => list.indexOf(value) === index);
  return `magnet:?xt=urn:btih:${encodeURIComponent(hash)}`
    + (displayName ? `&dn=${encodeURIComponent(displayName)}` : "")
    + trackers.map((source) => `&tr=${encodeURIComponent(source)}`).join("");
}

export const TMovieResolver = {

  isEnabled() {
    const settings = TMovieSettingsStore.get();
    return Boolean(settings.enabled && settings.serverUrl && settings.paired);
  },

  // A torrent stream with no direct URL — what TMovie can resolve.
  canResolveStream(stream = {}) {
    return !getStreamUrl(stream) && Boolean(stream.infoHash || torrentMagnetUri(stream));
  },

  // Fire-and-forget: if url is a TMovie torrent-stream URL for the configured
  // server, tell the server to drop the torrent and delete its cached data.
  // Called when playback stops so the server disk does not fill up.
  stopForUrl(url) {
    try {
      const settings = TMovieSettingsStore.get();
      const serverUrl = settings.serverUrl;
      const text = String(url || "");
      if (!serverUrl || !text || text.indexOf("/api/v1/torrent/stream") === -1) {
        return;
      }
      if (text.indexOf(serverUrl) !== 0) {
        return;
      }
      const query = text.slice(text.indexOf("?") + 1);
      const params = new URLSearchParams(query);
      let infoHash = String(params.get("ih") || "").trim();
      if (!infoHash) {
        const magnet = String(params.get("magnet") || "");
        const match = magnet.match(/btih:([a-zA-Z0-9]+)/i);
        infoHash = match ? match[1] : "";
      }
      if (!infoHash) {
        return;
      }
      void TMovieApi.stopTorrent({ serverUrl, token: settings.apiToken, infoHash }).catch(() => {});
    } catch {
      /* ignore */
    }
  },

  async resolve(stream = {}, { season = null, episode = null } = {}) {
    const settings = TMovieSettingsStore.get();
    if (!settings.enabled || !settings.serverUrl || !settings.paired) {
      return { status: "failed", stream: null };
    }
    const magnetUri = buildMagnetUri(stream);
    if (!magnetUri && !stream.infoHash) {
      return { status: "failed", stream: null };
    }
    const result = await TMovieApi.resolveTorrent({
      serverUrl: settings.serverUrl,
      token: settings.apiToken,
      infoHash: stream.infoHash || null,
      magnetUri,
      fileIdx: stream.fileIdx ?? null,
      season,
      episode
    });
    if (result.status !== "success" || !result.url) {
      return { status: "failed", stream: null };
    }
    return {
      status: "success",
      stream: {
        url: result.url,
        behaviorHints: stream.behaviorHints || null,
        raw: stream.raw || null
      }
    };
  }

};
