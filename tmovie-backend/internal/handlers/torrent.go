package handlers

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"tmovie/internal/config"

	"github.com/gin-gonic/gin"
)

// Version is reported by /api/v1/setup/status for the app's pairing check.
const Version = "0.3.0"

// publicBase returns the externally reachable base URL for building play URLs.
func (a *API) publicBase(c *gin.Context) string {
	if base := strings.TrimSpace(a.Cfg.PublicBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func torrentSource(c *gin.Context) string {
	if m := strings.TrimSpace(c.Query("magnet")); m != "" {
		return m
	}
	return strings.TrimSpace(c.Query("ih"))
}

// torrentResolve adds a magnet/infohash and returns the chosen file + a play URL.
func (a *API) torrentResolve(c *gin.Context) {
	if a.Torrent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "torrent engine disabled (set TORRENT_DIR)"})
		return
	}
	src := torrentSource(c)
	if src == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing magnet or ih"})
		return
	}
	fileIdx := config.Atoi(c.DefaultQuery("f", "-1"), -1)
	season := config.Atoi(c.Query("season"), 0)
	episode := config.Atoi(c.Query("episode"), 0)
	res, err := a.Torrent.Resolve(src, fileIdx, season, episode)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	// Build a short, extension-bearing play URL: the torrent is already loaded by
	// Resolve(), so /stream can reuse it by infohash (no giant magnet in the URL).
	// The trailing filename gives avplay the .mkv/.mp4 extension it needs.
	name := filepath.Base(res.Filename)
	if name == "" || name == "." {
		name = "video"
	}
	playURL := a.publicBase(c) + "/api/v1/torrent/stream/" + url.PathEscape(name) +
		"?ih=" + url.QueryEscape(res.InfoHash) + "&f=" + strconv.Itoa(res.FileIdx)
	c.JSON(http.StatusOK, gin.H{
		"playUrl":  playURL,
		"fileIdx":  res.FileIdx,
		"filename": res.Filename,
		"infoHash": res.InfoHash,
		"files":    res.Files,
	})
}

// torrentStream serves the chosen file (range + stream-while-download).
func (a *API) torrentStream(c *gin.Context) {
	if a.Torrent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "torrent engine disabled"})
		return
	}
	src := torrentSource(c)
	if src == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing magnet or ih"})
		return
	}
	fileIdx := config.Atoi(c.DefaultQuery("f", "-1"), -1)
	season := config.Atoi(c.Query("season"), 0)
	episode := config.Atoi(c.Query("episode"), 0)
	if err := a.Torrent.Serve(c.Writer, c.Request, src, fileIdx, season, episode); err != nil {
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
	}
}

func (a *API) torrentActive(c *gin.Context) {
	if a.Torrent == nil {
		c.JSON(http.StatusOK, gin.H{"torrents": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"torrents": a.Torrent.Active()})
}

func (a *API) torrentStop(c *gin.Context) {
	if a.Torrent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "torrent engine disabled"})
		return
	}
	dropped := a.Torrent.Stop(strings.TrimSpace(c.Query("ih")))
	c.JSON(http.StatusOK, gin.H{"dropped": dropped})
}

// setupStatus lets the app confirm the server is installed and discoverable.
// The "service" marker lets a LAN sweep recognise this host immediately.
func (a *API) setupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":       "tmovie",
		"installed":     true,
		"paired":        true,
		"tokenRequired": strings.TrimSpace(a.Cfg.APIToken) != "",
		"torrent":       a.Torrent != nil,
		"version":       Version,
	})
}

// requireToken gates control routes when an API token is configured.
func (a *API) requireToken(c *gin.Context) {
	want := strings.TrimSpace(a.Cfg.APIToken)
	if want == "" {
		c.Next()
		return
	}
	got := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if got == "" {
		got = strings.TrimSpace(c.Query("token"))
	}
	if got != want {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}
