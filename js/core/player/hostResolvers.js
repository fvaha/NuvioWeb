// Host resolvers: turn a file-host "embed" URL (pixeldrain, streamtape, streamwish,
// filemoon, doodstream, mixdrop ...) into a DIRECT playable mp4/m3u8 URL.
//
// Plugin providers sometimes return a host page URL instead of a direct stream. The TV
// player (AVPlay) cannot play an HTML embed page, so those streams silently fail. This
// module resolves the common hosts the way self-hosted apps (BeeTV/CloudStream) do —
// no account, no debrid: fetch the host page and extract the direct link.
//
// Runtime: packaged TV app, cross-origin fetch allowed, Chromium 47. Pure fetch + regex
// + a p,a,c,k,e,d unpacker. Each resolver returns a direct URL (with optional headers)
// or null; on null/timeout the caller keeps the original URL.

const RESOLVE_TIMEOUT_MS = 12000;
const UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36";

function withTimeout(promise, ms) {
  return new Promise((resolve) => {
    let done = false;
    const t = setTimeout(() => { if (!done) { done = true; resolve(null); } }, ms);
    Promise.resolve(promise).then(
      (v) => { if (!done) { done = true; clearTimeout(t); resolve(v); } },
      () => { if (!done) { done = true; clearTimeout(t); resolve(null); } }
    );
  });
}

async function getText(url, headers) {
  const res = await fetch(url, { headers: Object.assign({ "User-Agent": UA }, headers || {}) });
  if (!res.ok) throw new Error("HTTP " + res.status);
  return res.text();
}

// Dean Edwards p,a,c,k,e,d unpacker — many hosts ship the stream URL inside a packed
// eval(function(p,a,c,k,e,d){...}) block.
function unpack(packed) {
  const m = packed.match(/}\s*\(\s*'(.*?)'\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*'(.*?)'\.split\('\|'\)/s);
  if (!m) return "";
  let payload = m[1];
  const radix = parseInt(m[2], 10);
  const count = parseInt(m[3], 10);
  const symtab = m[4].split("|");
  const toBase = (n) => {
    let s = "";
    do { s = String.fromCharCode(n % radix > 35 ? (n % radix) + 29 : (n % radix).toString(36).charCodeAt(0)); n = Math.floor(n / radix); } while (n);
    // fallback to standard base conversion
    return s;
  };
  const dict = {};
  for (let i = count - 1; i >= 0; i--) {
    const key = i.toString(radix);
    dict[key] = symtab[i] && symtab[i].length ? symtab[i] : key;
  }
  payload = payload.replace(/\\'/g, "'").replace(/\\\\/g, "\\");
  return payload.replace(/\b\w+\b/g, (w) => dict[w] || w);
}

function firstM3u8OrMp4(text) {
  const m = text.match(/(https?:\\?\/\\?\/[^\s"'<>\\]+\.(?:m3u8|mp4)[^\s"'<>\\]*)/i);
  return m ? m[1].replace(/\\\//g, "/") : null;
}

// --- pixeldrain ------------------------------------------------------------
// cdn.pixeldrain.* /<id> or pixeldrain.com/u/<id> -> direct /api/file/<id>.
// The .eu.cc CDN 302-redirects to a direct video file; follow it / normalize.
async function resolvePixeldrain(url) {
  const id = (url.match(/pixeldrain[^/]*\/(?:u\/|api\/file\/)?([a-zA-Z0-9]+)/) || [])[1]
    || (url.match(/\/([a-zA-Z0-9]{6,})$/) || [])[1];
  if (!id) return null;
  // Follow one redirect hop to land on the direct CDN file.
  try {
    const res = await fetch(url, { headers: { "User-Agent": UA }, redirect: "follow" });
    if (res.url && /\/api\/file\//.test(res.url)) return { url: res.url };
  } catch (_) { /* fall through */ }
  return { url: "https://pixeldrain.com/api/file/" + id + "?download" };
}

// --- streamtape ------------------------------------------------------------
async function resolveStreamtape(url) {
  const id = (url.match(/streamtape\.[^/]+\/(?:e|v)\/([^/?#]+)/) || [])[1];
  if (!id) return null;
  const html = await getText("https://streamtape.com/e/" + id);
  // norobotlink token is split across two strings to defeat scrapers.
  const m = html.match(/robotlink'\)\.innerHTML\s*=\s*(['"])(.*?)\1\s*\+\s*(['"])(.*?)\3/);
  if (!m) return null;
  let link = (m[2] + m[4]).replace(/^\/\//, "https://");
  // token tail after the substring marker
  if (!/^https?:/.test(link)) link = "https:" + link;
  return { url: link, headers: { Referer: "https://streamtape.com/" } };
}

// --- streamwish / filemoon / vidhide family (packed JS -> m3u8) ------------
async function resolveStreamwishFamily(url) {
  const html = await getText(url.replace(/\/(?:d|f)\//, "/e/"), { Referer: url });
  const packed = (html.match(/eval\(function\(p,a,c,k,e,d\)[\s\S]*?\)\)\)/) || [])[0];
  if (packed) {
    const direct = firstM3u8OrMp4(unpack(packed));
    if (direct) return { url: direct, headers: { Referer: url } };
  }
  const inline = firstM3u8OrMp4(html);
  return inline ? { url: inline, headers: { Referer: url } } : null;
}

// --- doodstream ------------------------------------------------------------
async function resolveDood(url) {
  const base = (url.match(/^(https?:\/\/[^/]+)/) || [])[1];
  const id = (url.match(/\/[ed]\/([^/?#]+)/) || [])[1];
  if (!base || !id) return null;
  const html = await getText(base + "/e/" + id, { Referer: base });
  const pass = (html.match(/\/pass_md5\/[^'"]+/) || [])[0];
  if (!pass) return null;
  const token = (pass.match(/[^/]+$/) || [])[0];
  const baseUrl = await getText(base + pass, { Referer: base + "/e/" + id });
  const rand = Array.from({ length: 10 }, () => "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"[Math.floor(Math.random() * 62)]).join("");
  const expiry = Date.now();
  return { url: baseUrl.trim() + rand + "?token=" + token + "&expiry=" + expiry, headers: { Referer: base + "/" } };
}

// --- mixdrop (packed -> MDCore.wurl) ---------------------------------------
async function resolveMixdrop(url) {
  const norm = url.replace(/\/f\//, "/e/");
  const html = await getText(norm);
  const packed = (html.match(/eval\(function\(p,a,c,k,e,d\)[\s\S]*?\)\)\)/) || [])[0];
  if (!packed) return null;
  const code = unpack(packed);
  const w = (code.match(/MDCore\.wurl\s*=\s*["']([^"']+)["']/) || [])[1];
  if (!w) return null;
  return { url: w.startsWith("//") ? "https:" + w : w, headers: { Referer: "https://mixdrop.ag/" } };
}

const RESOLVERS = [
  { test: /pixeldrain/i, fn: resolvePixeldrain },
  { test: /streamtape|stape|tapecontent|streamadblock/i, fn: resolveStreamtape },
  { test: /streamwish|filemoon|vidhide|filelions|streamruby|swdyu|wishonly|streamhg|earnvids|kerapoxy|smoothpre/i, fn: resolveStreamwishFamily },
  { test: /dood|d000d|ds2play|dooood|doods/i, fn: resolveDood },
  { test: /mixdrop|mxdrop|mdfx/i, fn: resolveMixdrop }
];

export const HostResolvers = {
  // True if `url` points at a known host page (not a direct stream) we can resolve.
  isResolvable(url) {
    const u = String(url || "");
    if (/\.(m3u8|mp4)(\?|$)/i.test(u)) return false;
    return RESOLVERS.some((r) => r.test.test(u));
  },

  // Resolve a single host URL to a direct stream. Returns { url, headers } or null.
  async resolve(url) {
    const entry = RESOLVERS.find((r) => r.test.test(String(url || "")));
    if (!entry) return null;
    try {
      return await withTimeout(entry.fn(url), RESOLVE_TIMEOUT_MS);
    } catch (e) {
      if (globalThis.console) console.log("[resolver] " + e.message);
      return null;
    }
  }
};
