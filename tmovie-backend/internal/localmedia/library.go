package localmedia

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"tmovie/internal/sources"
)

// Library maps TMDB ids to video files under a single root (your filmovi tree).
// JSON format: see data/local_media_map.example.json
type Library struct {
	root    string
	mapPath string

	mu     sync.RWMutex
	movies map[int]string // TMDB movie id -> rel path from root
	tv     map[string]string
}

type mapFile struct {
	Movies map[string]string `json:"movies"`
	TV     map[string]string `json:"tv"`
}

func NewLibrary(root, mapPath string) (*Library, error) {
	root = strings.TrimSpace(root)
	mapPath = strings.TrimSpace(mapPath)
	if root == "" || mapPath == "" {
		return nil, nil
	}
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	lib := &Library{root: root, mapPath: mapPath}
	if err := lib.Reload(); err != nil {
		return nil, err
	}
	return lib, nil
}

func (lib *Library) Enabled() bool { return lib != nil && lib.root != "" && lib.mapPath != "" }

func (lib *Library) Reload() error {
	raw, err := os.ReadFile(lib.mapPath)
	if err != nil {
		return fmt.Errorf("local media map %s: %w", lib.mapPath, err)
	}
	var mf mapFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		return fmt.Errorf("local media map json: %w", err)
	}
	mov := make(map[int]string)
	for k, v := range mf.Movies {
		id, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || id <= 0 {
			continue
		}
		rel, ok := cleanRelPath(v)
		if !ok {
			continue
		}
		mov[id] = rel
	}
	tv := make(map[string]string)
	for k, v := range mf.TV {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		rel, ok := cleanRelPath(v)
		if !ok {
			continue
		}
		tv[key] = rel
	}
	lib.mu.Lock()
	lib.movies = mov
	lib.tv = tv
	lib.mu.Unlock()
	return nil
}

func cleanRelPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "..") {
		return "", false
	}
	s = filepath.FromSlash(s)
	s = strings.TrimLeft(s, string(os.PathSeparator))
	if s == "" {
		return "", false
	}
	return s, true
}

func (lib *Library) absUnderRoot(rel string) (string, bool) {
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	full, err := filepath.Abs(filepath.Join(lib.root, rel))
	if err != nil {
		return "", false
	}
	rootAbs, err := filepath.Abs(lib.root)
	if err != nil {
		return "", false
	}
	sep := string(os.PathSeparator)
	if full != rootAbs && !strings.HasPrefix(full+sep, rootAbs+sep) {
		return "", false
	}
	if st, err := os.Stat(full); err != nil || st.IsDir() {
		return "", false
	}
	return full, true
}

func (lib *Library) MovieAbsPath(id int) (string, bool) {
	lib.mu.RLock()
	rel, ok := lib.movies[id]
	lib.mu.RUnlock()
	if !ok {
		return "", false
	}
	return lib.absUnderRoot(rel)
}

func tvKey(showID, season, episode int) string {
	return fmt.Sprintf("%d:%d:%d", showID, season, episode)
}

func (lib *Library) TVAbsPath(showID, season, episode int) (string, bool) {
	lib.mu.RLock()
	rel, ok := lib.tv[tvKey(showID, season, episode)]
	lib.mu.RUnlock()
	if !ok {
		return "", false
	}
	return lib.absUnderRoot(rel)
}

func (lib *Library) FirstSRTBesideVideo(videoAbs string) (string, bool) {
	dir := filepath.Dir(videoAbs)
	base := strings.TrimSuffix(filepath.Base(videoAbs), filepath.Ext(videoAbs))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var srtFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".srt") {
			continue
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		if strings.EqualFold(stem, base) || strings.HasPrefix(strings.ToLower(stem), strings.ToLower(base)) {
			return p, true
		}
		srtFiles = append(srtFiles, p)
	}
	if len(srtFiles) == 1 {
		return srtFiles[0], true
	}
	return "", false
}

// MoviePlaySource returns a direct PlaySource for AVPlay when this movie is mapped.
func (lib *Library) MoviePlaySource(origin string, id int) (sources.PlaySource, bool) {
	if _, ok := lib.MovieAbsPath(id); !ok {
		return sources.PlaySource{}, false
	}
	u := strings.TrimRight(origin, "/") + "/api/v1/local/stream/movie/" + strconv.Itoa(id)
	return sources.PlaySource{
		Name:     "This server (local file)",
		URL:      u,
		Kind:     sources.SourceDirect,
		Priority: 1,
		Provider: "local",
	}, true
}

func (lib *Library) TVPlaySource(origin string, showID, season, episode int) (sources.PlaySource, bool) {
	if _, ok := lib.TVAbsPath(showID, season, episode); !ok {
		return sources.PlaySource{}, false
	}
	u := fmt.Sprintf("%s/api/v1/local/stream/tv/%d/%d/%d", strings.TrimRight(origin, "/"), showID, season, episode)
	return sources.PlaySource{
		Name:     "This server (local file)",
		URL:      u,
		Kind:     sources.SourceDirect,
		Priority: 1,
		Provider: "local",
	}, true
}

func (lib *Library) HasMovieSubtitle(id int) bool {
	v, ok := lib.MovieAbsPath(id)
	if !ok {
		return false
	}
	_, ok = lib.FirstSRTBesideVideo(v)
	return ok
}

func (lib *Library) HasTVSubtitle(showID, season, episode int) bool {
	v, ok := lib.TVAbsPath(showID, season, episode)
	if !ok {
		return false
	}
	_, ok = lib.FirstSRTBesideVideo(v)
	return ok
}

// ServeVideo writes the file with Range support (Seek).
func (lib *Library) ServeVideo(w http.ResponseWriter, r *http.Request, abs string) {
	http.ServeFile(w, r, abs)
}

func (lib *Library) ServeSubtitle(w http.ResponseWriter, r *http.Request, abs string) {
	w.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
	http.ServeFile(w, r, abs)
}
