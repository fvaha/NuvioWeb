# TMovie backend

Self-hosted companion server for the NuvioWeb TV app (Tizen / webOS / browser).
It runs on your own LAN and gives the app:

- **Torrent streaming resolver** (a self-hosted "debrid" replacement) — takes a
  magnet/infohash and streams the file over HTTP while it downloads (range/seek),
  via `anacrolix/torrent`. Auto-evicts idle torrents and caps disk use.
- **Subtitles addon** (Stremio `subtitles` resource) — **SubDL + OpenSubtitles**,
  matched by IMDb id (so they work on torrent/HTTP with no file hash). Serves
  clean UTF-8 SRT (converts `.sub` MicroDVD, fixes Windows-1250/ISO-8859-2,
  strips ASS tags).
- **Catalog/metadata** via TMDB/OMDB and optional **local media** streaming.

> **Linux only.** Built and run on Linux x86_64 (uses a systemd service). Other
> OSes are not supported by the helper scripts.

## Requirements

- Linux x86_64
- Go 1.23+ (1.25 works)
- A **TMDB API key** (required for catalog/search). Optional: OMDB, OpenSubtitles
  (api key + account), SubDL (api key) for the extra features.

## Setup

```bash
cp env.example .env          # then edit .env and fill in your keys
./HELPERS/install.sh         # builds bin/tmovie, writes systemd unit, starts it
```

Or run manually without systemd:

```bash
go build -o bin/tmovie ./cmd/server
./bin/tmovie                 # listens on :8080 (PORT)
```

## Configuration (`.env`)

| Key | Purpose |
|-----|---------|
| `PORT` | HTTP port (default 8080) |
| `TMDB_API_KEY` | TMDB key (required) |
| `OMDB_API_KEY` | OMDB key (optional, IMDb ratings) |
| `OPENSUBTITLES_API_KEY` | OpenSubtitles API key (subtitles) |
| `OPENSUBTITLES_USERNAME` / `OPENSUBTITLES_PASSWORD` | OpenSubtitles account (optional; search+download work with the API key alone) |
| `SUBDL_API_KEY` | SubDL API key (subtitles) — free at subdl.com |
| `SUBDL_LANGUAGES` | Comma list, e.g. `EN,HR,SR,BS` |
| `TORRENT_DIR` | Enables the torrent resolver; data dir for downloads |
| `TORRENT_IDLE_MINUTES` | Evict a torrent this long after reads stop (default 10) |
| `TORRENT_MAX_GB` | Hard cap on cached torrent data (default 20, 0 = off) |
| `TMOVIE_API_TOKEN` | Optional bearer token gating control routes (LAN: leave empty) |

Never commit `.env` — it holds your keys (it is in `.gitignore`).

## Pair with the app

In NuvioWeb on the TV:

1. **Settings → Integration → TMovie Server → Find server on my network**
   (auto-discovers this server on the LAN), or enter `http://YOUR_SERVER_IP:8080`.
2. **Settings → Addons** (or the phone manager) → add the subtitles addon:
   `http://YOUR_SERVER_IP:8080/manifest.json`.

The app plays torrents through this server and pulls SubDL + OpenSubtitles
subtitles from it.

## Key endpoints

- `GET /api/v1/torrent/resolve?magnet=…|ih=…&f=&season=&episode=` → `{ playUrl, fileIdx, files }`
- `GET /api/v1/torrent/stream/<name>?ih=…&f=…` → video (HTTP range, stream-while-download)
- `GET /manifest.json` + `GET /subtitles/:type/:id.json` → Stremio subtitles (SubDL + OpenSubtitles)
- `GET /api/v1/setup/status` → install/pair status
