package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"tmovie/internal/config"
	"tmovie/internal/subdl"
	"tmovie/internal/subtitles"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
)

// stremioManifest advertises TMovie as a Stremio subtitles addon (SubDL-backed).
// The app adds this server's URL in Settings -> Addons; it runs alongside OpenSubtitles.
func (a *API) stremioManifest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":          "tmovie.subdl.subtitles",
		"version":     Version,
		"name":        "TMovie SubDL",
		"description": "Subtitles via SubDL — matches by IMDb id, so it works for torrents (no file hash needed).",
		"resources":   []string{"subtitles"},
		"types":       []string{"movie", "series"},
		"catalogs":    []interface{}{},
		"idPrefixes":  []string{"tt"},
	})
}

func stripJSONSuffix(s string) string {
	return strings.TrimSuffix(strings.TrimSpace(s), ".json")
}

// stremioSubtitles answers /subtitles/:type/:id(.json) with SubDL + OpenSubtitles
// results, both matched by IMDb id (so they work on torrent/http with no file hash).
func (a *API) stremioSubtitles(c *gin.Context) {
	id := stripJSONSuffix(c.Param("id"))
	imdb := id
	season, episode := 0, 0
	if parts := strings.Split(id, ":"); len(parts) >= 3 {
		imdb = parts[0]
		season = config.Atoi(parts[1], 0)
		episode = config.Atoi(parts[2], 0)
	}
	base := a.publicBase(c)
	out := make([]gin.H, 0, 32)

	// --- SubDL ---
	if a.SubDL != nil && a.SubDL.Enabled() {
		if subs, err := a.SubDL.Search(imdb, season, episode); err == nil {
			for i, s := range subs {
				name := s.ReleaseName
				if name == "" {
					name = s.Name
				}
				if name == "" {
					name = "subtitle"
				}
				fileURL := base + "/api/v1/subtitle/file/" + url.PathEscape(sanitizeName(name)+".srt") +
					"?u=" + url.QueryEscape(s.ZipPath)
				out = append(out, gin.H{
					"id":   "subdl-" + strconv.Itoa(i) + "-" + s.Lang,
					"url":  fileURL,
					"lang": s.Lang,
				})
			}
		}
	}

	// --- OpenSubtitles (by IMDb id) ---
	if a.Subs != nil && a.Subs.Enabled() {
		langs := strings.ToLower(strings.TrimSpace(a.Cfg.SubDLLanguages))
		if langs == "" {
			langs = "en"
		}
		var tracks []subtitles.Track
		var oerr error
		if season > 0 && episode > 0 {
			tracks, oerr = a.Subs.SearchEpisode(imdb, season, episode, langs)
		} else {
			tracks, oerr = a.Subs.SearchMovie(imdb, langs)
		}
		if oerr == nil {
			for i, tr := range tracks {
				if tr.FileID == 0 {
					continue
				}
				name := tr.Release
				if name == "" {
					name = tr.FileName
				}
				if name == "" {
					name = "subtitle"
				}
				fileURL := base + "/api/v1/subtitle/os/" + url.PathEscape(sanitizeName(name)+".srt") +
					"?file_id=" + strconv.FormatInt(tr.FileID, 10)
				out = append(out, gin.H{
					"id":   "os-" + strconv.Itoa(i) + "-" + strings.ToLower(tr.Language),
					"url":  fileURL,
					"lang": strings.ToLower(tr.Language),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"subtitles": out})
}

// subtitleFileOS downloads an OpenSubtitles file (by file_id) and serves it as
// UTF-8 SRT (with MicroDVD conversion), so OpenSubtitles works like SubDL.
func (a *API) subtitleFileOS(c *gin.Context) {
	if a.Subs == nil || !a.Subs.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "opensubtitles not configured"})
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Query("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}
	dest := filepath.Join(a.Cfg.PublicDir, "subtitles")
	res, derr := a.Subs.DownloadToDisk(fileID, dest, fmt.Sprintf("os_%d.srt", fileID))
	if derr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": derr.Error()})
		return
	}
	raw, rerr := os.ReadFile(filepath.Join(a.Cfg.PublicDir, res.LocalRelPath))
	if rerr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": rerr.Error()})
		return
	}
	text := toUTF8Subtitle(raw)
	if converted, ok := microDVDToSRT(text); ok {
		text = converted
	}
	c.Header("Content-Type", "application/x-subrip; charset=utf-8")
	c.Header("Access-Control-Allow-Origin", "*")
	c.String(http.StatusOK, cleanSubtitle(text))
}

// subtitleFile downloads a SubDL zip, extracts the first .srt, and serves it.
func (a *API) subtitleFile(c *gin.Context) {
	zipPath := strings.TrimSpace(c.Query("u"))
	if zipPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing u"})
		return
	}
	resp, err := http.Get(subdl.DownloadBase() + zipPath)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "subdl download http " + strconv.Itoa(resp.StatusCode)})
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "bad zip"})
		return
	}
	// Pick the largest subtitle file (skips tiny sample/forced files). Accept .sub
	// too (MicroDVD) — many Balkan SubDL subs ship as .sub, not .srt.
	var best *zip.File
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !hasSubtitleExt(name) {
			continue
		}
		if best == nil || f.UncompressedSize64 > best.UncompressedSize64 {
			best = f
		}
	}
	if best == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no subtitle in archive"})
		return
	}
	rc, oerr := best.Open()
	if oerr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": oerr.Error()})
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(rc, 16<<20))
	rc.Close()
	text := toUTF8Subtitle(raw)
	// MicroDVD (.sub: {start}{end}text) is frame-based; the player only parses
	// SRT/VTT timestamps, so convert it.
	if converted, ok := microDVDToSRT(text); ok {
		text = converted
	}
	c.Header("Content-Type", "application/x-subrip; charset=utf-8")
	c.Header("Access-Control-Allow-Origin", "*")
	c.String(http.StatusOK, cleanSubtitle(text))
}

func hasSubtitleExt(name string) bool {
	for _, ext := range []string{".srt", ".vtt", ".sub", ".ssa", ".ass"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

var microDVDLine = regexp.MustCompile(`^\{(\d+)\}\{(\d+)\}(.*)$`)
var microDVDTag = regexp.MustCompile(`\{[a-zA-Z]:[^}]*\}`)
var assOverrideTag = regexp.MustCompile(`\{\\[^}]*\}`)

// cleanSubtitle strips ASS/SSA override tags ({\an5}, {\i1}…) that the player
// would otherwise render as literal text.
func cleanSubtitle(text string) string {
	return assOverrideTag.ReplaceAllString(text, "")
}

// microDVDToSRT converts MicroDVD .sub to SRT. Returns (srt, true) if input was
// MicroDVD. FPS comes from a leading {1}{1}<fps> line if present, else 23.976.
func microDVDToSRT(text string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	fps := 23.976
	type cue struct{ start, end float64; body string }
	cues := make([]cue, 0, len(lines))
	for _, line := range lines {
		m := microDVDLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		f1, _ := strconv.Atoi(m[1])
		f2, _ := strconv.Atoi(m[2])
		body := microDVDTag.ReplaceAllString(m[3], "")
		body = strings.ReplaceAll(body, "|", "\n")
		// First line "{1}{1}25.000" (or {0}{0}) only declares FPS.
		if len(cues) == 0 && f1 <= 1 && f2 <= 1 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(body), 64); err == nil && v > 1 {
				fps = v
				continue
			}
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		cues = append(cues, cue{start: float64(f1) / fps, end: float64(f2) / fps, body: body})
	}
	if len(cues) == 0 {
		return "", false
	}
	var b strings.Builder
	for i, c := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(c.start), srtTime(c.end), c.body)
	}
	return b.String(), true
}

func srtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	ms := int64(sec*1000 + 0.5)
	h := ms / 3600000
	ms -= h * 3600000
	m := ms / 60000
	ms -= m * 60000
	s := ms / 1000
	ms -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// toUTF8Subtitle normalizes subtitle bytes to UTF-8. Many Balkan subs are
// Windows-1250 / ISO-8859-2; without this they render empty or garbled.
func toUTF8Subtitle(raw []byte) string {
	// Strip UTF-8 BOM.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(raw) {
		return string(raw)
	}
	// Try Windows-1250 (most common ex-Yu), then ISO-8859-2 as fallback.
	if out, err := charmap.Windows1250.NewDecoder().Bytes(raw); err == nil && utf8.Valid(out) {
		return string(out)
	}
	if out, err := charmap.ISO8859_2.NewDecoder().Bytes(raw); err == nil {
		return string(out)
	}
	return string(raw)
}

func sanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '?', '#', '&', '%', ' ':
			return '_'
		}
		return r
	}, s)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}
