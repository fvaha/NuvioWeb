# TMovie na Samsung TV (Tizen) — instalacija i „Native Proxy (VidSrc)”

## Izvorni kod

- Tizen aplikacija: `client-tizen/` (`index.html`, `js/app.js`, `config.xml`, …)
- Backend (Go): repo root, binarni fajl na serveru: `~/tmovie/bin/tmovie`
- Paket za TV: nakon `tizen package` — `dist/TMovie.wgt/TMovie.wgt` (unutar podfoldera `TMovie.wgt/`)

## Backend na serveru (`YOUR_SERVER_IP`)

1. Deploy / build: iz Mac-a `HELPERS/deploy-remote.sh` (ili ručno `go build` u `~/tmovie`).
2. `.env` u `~/tmovie` — API ključevi + po želji `PUBLIC_BASE_URL=http://YOUR_SERVER_IP:8080`.
3. Pokretanje API-ja (ako nemaš passwordless `systemctl`):  
   `cd ~/tmovie && set -a && . ./.env && set +a && nohup ./bin/tmovie >> run.log 2>&1 &`
4. Provera: `curl http://127.0.0.1:8080/api/v1/health` → `200`.

## Scraper (obavezno za „Native Proxy (VidSrc)”)

Go endpoint `/api/v1/proxy/init` zove lokalno **`http://127.0.0.1:3000/extract`** (vidi `internal/handlers/proxy.go`).

Na serveru:

```bash
cd ~/tmovie/scraper && npm install --omit=dev
nohup node index.js >> ~/tmovie/scraper-run.log 2>&1 &
```

Log: `Scraper service running on port 3000`. Port **3000** ne sme biti zauzet drugim servisom.

## Tizen aplikacija na TV

1. Na Mac-u: Developer mode na TV, `sdb connect <TV_IP>:26101` (ili port koji TV prikaže).
2. Provera: `/Users/vaha/tizenstudio/tools/sdb devices` — mora biti `device`, ne `offline`.
3. Build + potpis + instalacija (profil `tMovie` u Tizen Certificate Manager):

```bash
cd client-tizen
/path/to/tizen build-web -- .
/path/to/tizen package -t wgt -s tMovie -o ../dist/TMovie.wgt -- .buildResult
/path/to/tizen install -n TMovie.wgt -s <TV>:<port> -- ../dist/TMovie.wgt/TMovie.wgt
```

Ako `tizen install` kaže **There is no connected target** — ponovo `sdb connect` i proveri mrežu / Developer IP na TV-u.

## URL backend-a u aplikaciji

U `client-tizen/js/config.js` postavi:

`apiBase: 'http://YOUR_SERVER_IP:8080'` (bez završnog `/`).

Relativni `sources[].url` koji počinju sa `/` aplikacija sama spaja sa `apiBase`.

## „Native Proxy (VidSrc)” u aplikaciji

- U listi servera izaberi **Native Proxy (VidSrc)** (prioritet u odnosu na iframe VidSrc).
- Reprodukcija ide kroz **Samsung AVPlay** kada je `kind: direct` i/ili direktan stream URL (vidi `client-tizen/js/app.js`).
- Ako strim ne krene: proveri da li **scraper** radi na **3000**, logove `scraper-run.log` i `run.log`, i da TMDB/IMDB parametri odgovaraju titlu.

## ID aplikacije na TV-u

`VAHATMOVI0.TMovie` (paket `VAHATMOVI0`).
