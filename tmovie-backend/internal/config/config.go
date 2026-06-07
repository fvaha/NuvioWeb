package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	TMDBAPIKey  string
	SQLitePath  string
	CORSOrigins []string
	OMDBAPIKey  string
	OSAPIKey    string
	OSUser      string
	OSPass      string
	PublicDir   string
	// PublicBaseURL is optional; e.g. http://YOUR_SERVER_IP:8080 — used for local file stream URLs in catalog when Host is missing.
	PublicBaseURL     string
	LocalLibraryRoot  string
	LocalMediaMapPath string
	FFmpegBin         string
	// TorrentDir enables the self-hosted torrent resolver when set.
	TorrentDir string
	// APIToken, when set, gates control routes (Bearer token).
	APIToken string
	// TorrentIdleMinutes: evict a torrent this long after reads stop (default 10).
	TorrentIdleMinutes int
	// TorrentMaxGB: hard cap on cached torrent data; 0 disables (default 20).
	TorrentMaxGB int
	// SubDLAPIKey enables the SubDL subtitles addon (imdb-id based; works for torrents).
	SubDLAPIKey string
	// SubDLLanguages: comma list queried from SubDL (default broad set).
	SubDLLanguages string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	key := os.Getenv("TMDB_API_KEY")
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "data/tmovie.db"
	}
	origins := os.Getenv("CORS_ORIGINS")
	var cors []string
	if origins != "" {
		for _, o := range strings.Split(origins, ",") {
			if s := strings.TrimSpace(o); s != "" {
				cors = append(cors, s)
			}
		}
	} else {
		cors = []string{"*"}
	}
	publicDir := os.Getenv("PUBLIC_DIR")
	if publicDir == "" {
		publicDir = "data/public"
	}
	return Config{
		Port:        port,
		TMDBAPIKey:  key,
		SQLitePath:  dbPath,
		CORSOrigins: cors,
		OMDBAPIKey:  os.Getenv("OMDB_API_KEY"),
		OSAPIKey:    os.Getenv("OPENSUBTITLES_API_KEY"),
		OSUser:      os.Getenv("OPENSUBTITLES_USERNAME"),
		OSPass:      os.Getenv("OPENSUBTITLES_PASSWORD"),
		PublicDir:   publicDir,
		PublicBaseURL:     strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")),
		LocalLibraryRoot:  strings.TrimSpace(os.Getenv("LOCAL_LIBRARY_ROOT")),
		LocalMediaMapPath: strings.TrimSpace(os.Getenv("LOCAL_MEDIA_MAP")),
		FFmpegBin:         strings.TrimSpace(os.Getenv("FFMPEG_BIN")),
		TorrentDir:        strings.TrimSpace(os.Getenv("TORRENT_DIR")),
		APIToken:          strings.TrimSpace(os.Getenv("TMOVIE_API_TOKEN")),
		TorrentIdleMinutes: Atoi(os.Getenv("TORRENT_IDLE_MINUTES"), 10),
		TorrentMaxGB:       Atoi(os.Getenv("TORRENT_MAX_GB"), 20),
		SubDLAPIKey:        strings.TrimSpace(os.Getenv("SUBDL_API_KEY")),
		SubDLLanguages:     strings.TrimSpace(os.Getenv("SUBDL_LANGUAGES")),
	}
}

func Atoi(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
