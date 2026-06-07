// LAN auto-discovery for the TMovie server.
//
// A Tizen web widget has no mDNS/SSDP/UDP, so we discover by HTTP: find the
// device's own LAN IPv4, then sweep the /24 hitting /api/v1/setup/status. The
// TMovie box answers fast (CORS *), absent hosts fail/time out. Chromium 47 has
// no AbortController, so timeouts are done with Promise.race.

const STATUS_PATH = "api/v1/setup/status";

function isPrivateIPv4(ip) {
  if (!/^\d{1,3}(\.\d{1,3}){3}$/.test(ip)) {
    return false;
  }
  const [a, b] = ip.split(".").map((n) => Number(n));
  if (a === 10) return true;
  if (a === 192 && b === 168) return true;
  if (a === 172 && b >= 16 && b <= 31) return true;
  return false;
}

// Read the LAN IP from Tizen's systeminfo (best-effort; may need a privilege).
function localIpViaTizen() {
  const tizen = globalThis.tizen;
  if (!tizen || !tizen.systeminfo) {
    return Promise.resolve(null);
  }
  const tryProperty = (property) => new Promise((resolve) => {
    try {
      tizen.systeminfo.getPropertyValue(
        property,
        (data) => resolve(data && isPrivateIPv4(String(data.ipAddress || "")) ? String(data.ipAddress) : null),
        () => resolve(null)
      );
    } catch {
      resolve(null);
    }
  });
  return tryProperty("ETHERNET_NETWORK").then((ip) => ip || tryProperty("WIFI_NETWORK"));
}

// Read the LAN IP by parsing WebRTC ICE candidates (works on Chromium 47).
function localIpViaWebRTC(timeoutMs = 1500) {
  return new Promise((resolve) => {
    const PeerConnection = globalThis.RTCPeerConnection
      || globalThis.webkitRTCPeerConnection
      || globalThis.mozRTCPeerConnection;
    if (!PeerConnection) {
      resolve(null);
      return;
    }
    let settled = false;
    let pc = null;
    const finish = (ip) => {
      if (settled) {
        return;
      }
      settled = true;
      try { pc && pc.close(); } catch { /* ignore */ }
      resolve(ip);
    };
    try {
      pc = new PeerConnection({ iceServers: [] });
      pc.createDataChannel("d");
      pc.onicecandidate = (event) => {
        const candidate = event && event.candidate && event.candidate.candidate;
        if (!candidate) {
          return;
        }
        const match = candidate.match(/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
        if (match && isPrivateIPv4(match[1])) {
          finish(match[1]);
        }
      };
      pc.createOffer().then((offer) => pc.setLocalDescription(offer)).catch(() => finish(null));
    } catch {
      finish(null);
      return;
    }
    setTimeout(() => finish(null), timeoutMs);
  });
}

function withTimeout(promise, timeoutMs) {
  return Promise.race([
    promise,
    new Promise((resolve) => setTimeout(() => resolve(null), timeoutMs))
  ]);
}

function probeHost(host, port, timeoutMs) {
  const base = `http://${host}:${port}`;
  let controller = null;
  if (globalThis.AbortController) {
    controller = new AbortController();
    setTimeout(() => { try { controller.abort(); } catch { /* ignore */ } }, timeoutMs);
  }
  const request = fetch(`${base}/${STATUS_PATH}`, {
    signal: controller ? controller.signal : undefined
  })
    .then((response) => (response.ok ? response.json() : null))
    .then((data) => {
      if (!data) {
        return null;
      }
      // Fast match on the service marker; fall back to capability fields.
      if (data.service === "tmovie" || (data.installed === true && data.torrent === true)) {
        return base;
      }
      return null;
    })
    .catch(() => null);
  return withTimeout(request, timeoutMs);
}

export const TMovieDiscovery = {

  async getLocalIPv4() {
    const tizenIp = await localIpViaTizen();
    if (tizenIp) {
      return tizenIp;
    }
    return localIpViaWebRTC();
  },

  // Sweep the local /24 and return the first reachable TMovie base URL, or null.
  // Uses an early-exit worker pool: resolves the instant any host matches, so a
  // server low in the range (e.g. .10) is found almost immediately.
  async discover({ port = 8080, timeoutMs = 600, concurrency = 48 } = {}) {
    const ip = await this.getLocalIPv4();
    if (!ip) {
      return { ip: null, serverUrl: null };
    }
    const prefix = ip.replace(/\.\d+$/, ".");
    const hosts = [];
    for (let last = 1; last <= 254; last += 1) {
      hosts.push(prefix + last);
    }
    return new Promise((resolve) => {
      let index = 0;
      let active = 0;
      let finished = false;
      const finish = (serverUrl) => {
        if (!finished) {
          finished = true;
          resolve({ ip, serverUrl });
        }
      };
      const pump = () => {
        if (finished) {
          return;
        }
        if (index >= hosts.length && active === 0) {
          finish(null);
          return;
        }
        while (active < concurrency && index < hosts.length && !finished) {
          const host = hosts[index];
          index += 1;
          active += 1;
          probeHost(host, port, timeoutMs).then((url) => {
            active -= 1;
            if (url) {
              finish(url);
            } else {
              pump();
            }
          });
        }
      };
      pump();
    });
  }

};
