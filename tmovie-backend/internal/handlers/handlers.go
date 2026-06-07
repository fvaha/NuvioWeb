package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"tmovie/internal/cache"
	"tmovie/internal/catalog"
	"tmovie/internal/config"
	"tmovie/internal/images"
	"tmovie/internal/localmedia"
	"tmovie/internal/models"
	"tmovie/internal/omdb"
	"tmovie/internal/subdl"
	"tmovie/internal/subtitles"
	"tmovie/internal/tmdb"
	"tmovie/internal/torrent"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type API struct {
	DB      *gorm.DB
	TMDB    *tmdb.Client
	OMDb    *omdb.Client
	Subs    *subtitles.Client
	Cfg     config.Config
	Local   *localmedia.Library
	Torrent *torrent.Engine
	SubDL   *subdl.Client
}

func (a *API) Register(r *gin.Engine) {
	// Stremio subtitles addon (SubDL-backed) at the root, addable via Settings -> Addons.
	r.GET("/manifest.json", a.stremioManifest)
	r.GET("/subtitles/:type/:id", a.stremioSubtitles)
	r.GET("/subtitles/:type/:id/*extra", a.stremioSubtitles)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		v1.GET("/status", a.status)
		v1.GET("/browse/trending/movies", a.browseTrendingMovies)
		v1.GET("/browse/trending/tv", a.browseTrendingTV)
		v1.GET("/browse/popular/movies", a.browsePopularMovies)
		v1.GET("/browse/popular/tv", a.browsePopularTV)
		v1.GET("/browse/home/hero", a.browseHomeHero)
		v1.GET("/search", a.search)
		v1.GET("/catalog/movie/:id", a.catalogMovie)
		v1.GET("/catalog/tv/:id", a.catalogTV)
		v1.GET("/tv/:id/seasons", a.tvSeasonsList)
		v1.GET("/tv/:id/season/:season/episodes", a.tvSeasonEpisodesList)
		v1.GET("/subtitles/pull", a.subtitlePull)
		v1.GET("/local/stream/movie/:id", a.localStreamMovie)
		v1.GET("/local/stream/tv/:id/:season/:episode", a.localStreamTV)
		v1.GET("/local/subs/movie/:id", a.localSubsMovie)
		v1.GET("/local/subs/tv/:id/:season/:episode", a.localSubsTV)
		v1.GET("/movie/:id", a.catalogMovieAlias)
		v1.GET("/tv/:id", a.catalogTVAlias)
		
		v1.GET("/proxy/init", a.proxyInit)
		v1.GET("/proxy/m3u8/play.m3u8", a.proxyM3U8)
		v1.GET("/proxy/ts/segment.ts", a.proxyTS)

		// Self-hosted torrent resolver (debrid replacement) + pairing status.
		v1.GET("/setup/status", a.setupStatus)
		v1.GET("/torrent/resolve", a.torrentResolve)
		v1.GET("/torrent/stream", a.torrentStream)
		// Same handler; the :name suffix just gives players a file extension.
		v1.GET("/torrent/stream/:name", a.torrentStream)
		v1.GET("/torrent/active", a.torrentActive)
		v1.POST("/torrent/stop", a.requireToken, a.torrentStop)

		// SubDL zip -> srt extractor (URL returned by /subtitles results).
		v1.GET("/subtitle/file/:name", a.subtitleFile)
		// OpenSubtitles file -> srt (URL returned by /subtitles results).
		v1.GET("/subtitle/os/:name", a.subtitleFileOS)
	}
}

func (a *API) search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("query"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing query"})
		return
	}
	page := config.Atoi(c.DefaultQuery("page", "1"), 1)
	results, err := a.TMDB.SearchMulti(q, page)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	filtered := make([]tmdb.MultiSearchResult, 0, len(results))
	for _, r := range results {
		if r.MediaType == "movie" || r.MediaType == "tv" {
			filtered = append(filtered, r)
		}
	}
	out := make([]gin.H, 0, len(filtered))
	for _, r := range filtered {
		title := r.Title
		if r.MediaType == "tv" && r.Name != "" {
			title = r.Name
		}
		out = append(out, gin.H{
			"id":                  r.ID,
			"media_type":          r.MediaType,
			"title":               r.Title,
			"name":                r.Name,
			"display_title":       title,
			"overview":            r.Overview,
			"poster_path":         r.PosterPath,
			"poster_url":          images.TMDB(r.PosterPath, "w342"),
			"release_date":        r.ReleaseDate,
			"first_air_date":      r.FirstAirDate,
			"original_title":      r.OriginalTitle,
			"release_or_air_date": pickAir(r),
		})
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "query": q, "results": out})
}

func pickAir(r tmdb.MultiSearchResult) string {
	if r.ReleaseDate != "" {
		return r.ReleaseDate
	}
	return r.FirstAirDate
}

func (a *API) catalogMovie(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	langs := strings.TrimSpace(c.Query("subtitle_langs"))
	payload, err := catalog.BuildMovie(a.TMDB, a.OMDb, a.Subs, id, langs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = upsertFromPayload(a.DB, payload)
	a.enrichMovieLocal(c, payload, id)
	c.JSON(http.StatusOK, payload)
}

func (a *API) catalogTV(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	season := config.Atoi(c.DefaultQuery("season", "1"), 1)
	episode := config.Atoi(c.DefaultQuery("episode", "1"), 1)
	langs := strings.TrimSpace(c.Query("subtitle_langs"))
	payload, err := catalog.BuildTV(a.TMDB, a.OMDb, a.Subs, id, season, episode, langs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = upsertFromPayload(a.DB, payload)
	a.enrichTVLocal(c, payload, id, season, episode)
	c.JSON(http.StatusOK, payload)
}

func (a *API) catalogMovieAlias(c *gin.Context) {
	a.catalogMovie(c)
}

func (a *API) catalogTVAlias(c *gin.Context) {
	a.catalogTV(c)
}

func (a *API) subtitlePull(c *gin.Context) {
	if a.Subs == nil || !a.Subs.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "opensubtitles not configured"})
		return
	}
	fileID, err := strconv.ParseInt(c.Query("file_id"), 10, 64)
	if err != nil || fileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "movie" && kind != "tv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be movie or tv"})
		return
	}
	tmdbID, err := strconv.Atoi(c.Query("tmdb_id"))
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tmdb_id"})
		return
	}
	dest := filepath.Join(a.Cfg.PublicDir, "subtitles")
	name := fmt.Sprintf("%s_%d_%d.srt", kind, tmdbID, fileID)
	res, err := a.Subs.DownloadToDisk(fileID, dest, name)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"file_url": "/files/" + res.LocalRelPath,
		"bytes":    res.Bytes,
	})
}

func upsertFromPayload(db *gorm.DB, payload map[string]interface{}) error {
	kind, _ := payload["kind"].(string)
	tm, ok := payload["tmdb"].(map[string]interface{})
	if !ok || tm == nil {
		return nil
	}
	pr, _ := payload["presentation"].(map[string]interface{})
	title := stringFrom(pr["title"])
	if title == "" {
		title = stringFrom(tm["title"])
	}
	if title == "" {
		title = stringFrom(tm["show_name"])
	}
	poster := stringFrom(tm["poster_path"])
	overview := stringFrom(tm["overview"])
	release := stringFrom(tm["release_date"])
	media := "movie"
	id := intFrom(tm["id"])
	if kind == "tv_episode" {
		media = "tv"
		release = stringFrom(tm["air_date"])
		id = intFrom(tm["show_id"])
	}
	if id == 0 {
		return nil
	}
	return cache.UpsertMovie(db, models.CachedMovie{
		TMDBID:      id,
		Title:       title,
		PosterPath:  poster,
		Overview:    overview,
		ReleaseDate: release,
		MediaType:   media,
	})
}

func intFrom(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	default:
		return 0
	}
}

func stringFrom(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprint(v)
}
