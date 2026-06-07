package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const APIVersion = "1.0.0"

func (a *API) status(c *gin.Context) {
	prevodiPath := os.Getenv("PREVODI_BACKEND_PY")
	if prevodiPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			prevodiPath = home + "/multimedia/PrevodiProjekat/backend.py"
		}
	}
	prevodiOK := false
	if prevodiPath != "" {
		if st, err := os.Stat(prevodiPath); err == nil && !st.IsDir() {
			prevodiOK = true
		}
	}

	dotenv := os.Getenv("TMOVIE_DOTENV")
	if dotenv == "" {
		if wd, err := os.Getwd(); err == nil {
			dotenv = wd + "/.env"
		}
	}
	dotenvOK := false
	if dotenv != "" {
		if st, err := os.Stat(dotenv); err == nil && !st.IsDir() {
			dotenvOK = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"service": "tmovie",
		"version": APIVersion,
		"integrations": gin.H{
			"tmdb":            a.Cfg.TMDBAPIKey != "",
			"omdb":            a.Cfg.OMDBAPIKey != "",
			"opensubtitles":   a.Subs != nil && a.Subs.Enabled(),
			"dotenv_file":     dotenvOK,
			"prevodi_backend": prevodiOK,
			"local_library":   a.Local != nil && a.Local.Enabled(),
			"ffmpeg_bin_set":  a.Cfg.FFmpegBin != "",
		},
		"paths": gin.H{
			"sqlite":               a.Cfg.SQLitePath,
			"public_dir":           a.Cfg.PublicDir,
			"dotenv_hint":          dotenv,
			"prevodi_backend_hint": prevodiPath,
			"local_library_root":   a.Cfg.LocalLibraryRoot,
			"local_media_map":      a.Cfg.LocalMediaMapPath,
			"public_base_url":      a.Cfg.PublicBaseURL,
			"ffmpeg_bin":           a.Cfg.FFmpegBin,
		},
		"endpoints": gin.H{
			"health":             "/api/v1/health",
			"status":             "/api/v1/status",
			"search":             "/api/v1/search?query=",
			"browse_trending_movies": "/api/v1/browse/trending/movies",
			"browse_trending_tv":     "/api/v1/browse/trending/tv",
			"browse_popular_movies":  "/api/v1/browse/popular/movies",
			"browse_popular_tv":      "/api/v1/browse/popular/tv",
			"browse_home_hero":       "/api/v1/browse/home/hero",
			"catalog_movie":      "/api/v1/catalog/movie/{tmdb_id}",
			"catalog_tv":         "/api/v1/catalog/tv/{tmdb_id}?season=&episode=",
			"tv_seasons":         "/api/v1/tv/{tmdb_id}/seasons",
			"tv_season_episodes": "/api/v1/tv/{tmdb_id}/season/{n}/episodes",
			"subtitles_pull":     "/api/v1/subtitles/pull?file_id=&kind=movie|tv&tmdb_id=",
			"local_stream_movie": "/api/v1/local/stream/movie/{tmdb_id}",
			"local_stream_tv":    "/api/v1/local/stream/tv/{tmdb_id}/{season}/{episode}",
			"local_subs_movie":   "/api/v1/local/subs/movie/{tmdb_id}",
			"local_subs_tv":      "/api/v1/local/subs/tv/{tmdb_id}/{season}/{episode}",
			"files_public":       "/files/",
			"dashboard":          "/dashboard/",
		},
		"clients": gin.H{
			"tizen":       "client-tizen/ (same REST API)",
			"android_tv":  "same API base URL /api/v1",
			"management":  "optional dashboard at /dashboard; primary control via apps + env",
		},
	})
}
