package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"tmovie/internal/config"
	"tmovie/internal/db"
	"tmovie/internal/handlers"
	"tmovie/internal/localmedia"
	"tmovie/internal/omdb"
	"tmovie/internal/subdl"
	"tmovie/internal/subtitles"
	"tmovie/internal/tmdb"
	"tmovie/internal/torrent"

	"github.com/gin-gonic/gin"
)

func main() {
	loadEnvFiles()
	cfg := config.Load()
	if cfg.TMDBAPIKey == "" {
		log.Print("warning: TMDB_API_KEY is empty; search and catalog will fail until set")
	}
	if cfg.OMDBAPIKey == "" {
		log.Print("info: OMDB_API_KEY is empty; IMDB rating fields may be missing")
	}
	if cfg.OSAPIKey == "" || cfg.OSUser == "" || cfg.OSPass == "" {
		log.Print("info: OpenSubtitles env vars incomplete; subtitle tracks/pull disabled until configured")
	}
	if cfg.FFmpegBin != "" {
		log.Printf("info: FFMPEG_BIN=%s (reserved for future transcode hooks; local play serves files as-is)", cfg.FFmpegBin)
	}

	if err := os.MkdirAll(filepath.Join(cfg.PublicDir, "subtitles"), 0o755); err != nil {
		log.Fatal(err)
	}

	gdb, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(cfg.CORSOrigins))
	r.Static("/files", cfg.PublicDir)
	mountDashboard(r)

	var lib *localmedia.Library
	if cfg.LocalLibraryRoot != "" && cfg.LocalMediaMapPath != "" {
		mapPath := cfg.LocalMediaMapPath
		if !filepath.IsAbs(mapPath) {
			if wd, err := os.Getwd(); err == nil {
				mapPath = filepath.Join(wd, mapPath)
			}
		}
		var lerr error
		lib, lerr = localmedia.NewLibrary(cfg.LocalLibraryRoot, mapPath)
		if lerr != nil {
			log.Fatalf("local media: %v", lerr)
		}
		log.Printf("local library: root=%s map=%s", cfg.LocalLibraryRoot, mapPath)
	} else if cfg.LocalLibraryRoot != "" || cfg.LocalMediaMapPath != "" {
		log.Print("warning: set both LOCAL_LIBRARY_ROOT and LOCAL_MEDIA_MAP to enable local file playback")
	}

	var teng *torrent.Engine
	if cfg.TorrentDir != "" {
		idleTTL := time.Duration(cfg.TorrentIdleMinutes) * time.Minute
		maxBytes := int64(cfg.TorrentMaxGB) * 1024 * 1024 * 1024
		te, terr := torrent.New(cfg.TorrentDir, idleTTL, maxBytes)
		if terr != nil {
			log.Printf("warning: torrent engine disabled: %v", terr)
		} else {
			teng = te
			log.Printf("torrent engine: dir=%s idle=%dm capGB=%d", cfg.TorrentDir, cfg.TorrentIdleMinutes, cfg.TorrentMaxGB)
		}
	} else {
		log.Print("info: TORRENT_DIR empty; torrent resolver disabled")
	}

	if cfg.SubDLAPIKey != "" {
		log.Printf("subdl subtitles: enabled (langs=%s)", cfg.SubDLLanguages)
	} else {
		log.Print("info: SUBDL_API_KEY empty; SubDL subtitles addon returns nothing until set")
	}

	api := &handlers.API{
		DB:      gdb,
		TMDB:    tmdb.New(cfg.TMDBAPIKey),
		OMDb:    omdb.New(cfg.OMDBAPIKey),
		Subs:    subtitles.New(cfg.OSAPIKey, cfg.OSUser, cfg.OSPass),
		Cfg:     cfg,
		Local:   lib,
		Torrent: teng,
		SubDL:   subdl.New(cfg.SubDLAPIKey, cfg.SubDLLanguages),
	}
	api.Register(r)

	addr := ":" + cfg.Port
	log.Printf("tmovie api listening on %s public=%s dashboard=%s", addr, cfg.PublicDir, dashboardDir())
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func dashboardDir() string {
	if p := os.Getenv("TMOVIE_DASHBOARD_DIR"); p != "" {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", "web", "dashboard")
	}
	return filepath.Join(wd, "web", "dashboard")
}

func mountDashboard(r *gin.Engine) {
	root := dashboardDir()
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		log.Printf("dashboard: disabled (missing %s)", root)
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service": "tmovie",
				"hint":    "API at /api/v1 — configure TMOVIE_DASHBOARD_DIR or add web/dashboard",
			})
		})
		return
	}
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard/index.html")
	})
	r.Static("/dashboard", root)
}

func loadEnvFiles() {
	paths := []string{}
	if p := os.Getenv("TMOVIE_DOTENV"); p != "" {
		paths = append(paths, p)
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, ".env"))
	}
	seen := map[string]struct{}{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if err := config.LoadDotEnv(p); err != nil {
			log.Printf("dotenv %s: %v", p, err)
		}
	}
}

func corsMiddleware(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := "*"
		if len(origins) > 0 && origins[0] != "*" {
			req := c.GetHeader("Origin")
			if len(origins) == 1 {
				allowed = origins[0]
			} else if req != "" {
				allowed = ""
				for _, o := range origins {
					if o == req {
						allowed = req
						break
					}
				}
				if allowed == "" {
					allowed = origins[0]
				}
			}
		}
		c.Header("Access-Control-Allow-Origin", allowed)
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
