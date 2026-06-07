package torrent

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

// Engine wraps a single anacrolix/torrent client and serves files over HTTP
// while they download (sequential readahead + range support). It is the
// self-hosted replacement for a debrid service: the app sends a magnet/infohash
// and plays the returned HTTP URL.
type Engine struct {
	client  *atorrent.Client
	dataDir string

	metaTimeout time.Duration
	idleTTL     time.Duration
	maxBytes    int64

	mu       sync.Mutex
	lastUsed map[metainfo.Hash]time.Time
}

// FileInfo describes one file inside a torrent.
type FileInfo struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
}

// ResolveResult is returned by Resolve: the chosen file plus the full file list.
type ResolveResult struct {
	InfoHash string     `json:"infoHash"`
	FileIdx  int        `json:"fileIdx"`
	Filename string     `json:"filename"`
	Files    []FileInfo `json:"files"`
}

// New starts a torrent client storing data under dataDir. Cached data is evicted
// when a torrent stops being read (idleTTL) and whenever the total exceeds
// maxBytes (oldest-first), so the disk cannot fill up. maxBytes <= 0 disables the cap.
func New(dataDir string, idleTTL time.Duration, maxBytes int64) (*Engine, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("torrent data dir is empty")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	if idleTTL <= 0 {
		idleTTL = 10 * time.Minute
	}
	cfg := atorrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir
	cfg.Seed = false
	cl, err := atorrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		client:      cl,
		dataDir:     dataDir,
		metaTimeout: 60 * time.Second,
		idleTTL:     idleTTL,
		maxBytes:    maxBytes,
		lastUsed:    map[metainfo.Hash]time.Time{},
	}
	go e.reaper()
	return e, nil
}

// Resolve adds the torrent (waiting for metadata) and reports the chosen file.
func (e *Engine) Resolve(magnetOrIH string, fileIdx, season, episode int) (*ResolveResult, error) {
	t, err := e.add(magnetOrIH)
	if err != nil {
		return nil, err
	}
	file, idx := chooseFile(t, fileIdx, season, episode)
	if file == nil {
		return nil, fmt.Errorf("no playable video file in torrent")
	}
	files := t.Files()
	out := make([]FileInfo, 0, len(files))
	for i, f := range files {
		out = append(out, FileInfo{Index: i, Name: f.DisplayPath(), Size: f.Length()})
	}
	return &ResolveResult{
		InfoHash: t.InfoHash().HexString(),
		FileIdx:  idx,
		Filename: file.DisplayPath(),
		Files:    out,
	}, nil
}

// Serve streams the chosen file via http.ServeContent (range + stream-while-download).
func (e *Engine) Serve(w http.ResponseWriter, r *http.Request, magnetOrIH string, fileIdx, season, episode int) error {
	t, err := e.add(magnetOrIH)
	if err != nil {
		return err
	}
	file, _ := chooseFile(t, fileIdx, season, episode)
	if file == nil {
		return fmt.Errorf("no playable video file in torrent")
	}
	// Pull the container header (start) and index (end) pieces first so players
	// like avplay can probe and start quickly instead of stalling on low-seed
	// torrents while they wait for the moov/cues at the file tail.
	prioritizeEnds(t, file)

	reader := file.NewReader()
	reader.SetReadahead(16 << 20) // 16 MiB lookahead for smooth playback
	reader.SetResponsive()
	defer reader.Close()
	// Mark the torrent in-use on every read/seek so the reaper keeps it alive
	// while playing, and evicts it shortly after playback stops.
	hash := t.InfoHash()
	touching := &touchingReader{inner: reader, touch: func() { e.touch(hash) }}
	http.ServeContent(w, r, file.DisplayPath(), time.Now(), touching)
	return nil
}

// touchingReader updates the engine's last-used timestamp on each access.
type touchingReader struct {
	inner interface {
		Read([]byte) (int, error)
		Seek(int64, int) (int64, error)
	}
	touch func()
}

func (tr *touchingReader) Read(p []byte) (int, error) {
	tr.touch()
	return tr.inner.Read(p)
}

func (tr *touchingReader) Seek(offset int64, whence int) (int64, error) {
	tr.touch()
	return tr.inner.Seek(offset, whence)
}

// Active lists currently loaded torrents.
func (e *Engine) Active() []gin_h {
	out := []gin_h{}
	for _, t := range e.client.Torrents() {
		name := t.InfoHash().HexString()
		if t.Info() != nil {
			name = t.Name()
		}
		out = append(out, gin_h{
			"infoHash": t.InfoHash().HexString(),
			"name":     name,
			"bytes":    t.BytesCompleted(),
		})
	}
	return out
}

// gin_h avoids importing gin here; handlers map it directly.
type gin_h = map[string]interface{}

// Stop drops one torrent (by hex infohash) or all when ih is empty.
func (e *Engine) Stop(ih string) int {
	dropped := 0
	if strings.TrimSpace(ih) == "" {
		for _, t := range e.client.Torrents() {
			e.dropTorrent(t)
			dropped++
		}
		return dropped
	}
	var h metainfo.Hash
	if err := h.FromHexString(strings.TrimSpace(ih)); err != nil {
		return 0
	}
	if t, ok := e.client.Torrent(h); ok {
		e.dropTorrent(t)
		dropped++
	}
	return dropped
}

func (e *Engine) add(magnetOrIH string) (*atorrent.Torrent, error) {
	s := strings.TrimSpace(magnetOrIH)
	if s == "" {
		return nil, fmt.Errorf("empty magnet/infohash")
	}
	var (
		t   *atorrent.Torrent
		err error
	)
	if strings.HasPrefix(strings.ToLower(s), "magnet:") {
		t, err = e.client.AddMagnet(s)
	} else {
		var h metainfo.Hash
		if err = h.FromHexString(strings.ToLower(s)); err != nil {
			return nil, fmt.Errorf("invalid infohash: %w", err)
		}
		t, _ = e.client.AddTorrentInfoHash(h)
	}
	if err != nil {
		return nil, err
	}
	select {
	case <-t.GotInfo():
	case <-time.After(e.metaTimeout):
		return nil, fmt.Errorf("timed out fetching torrent metadata")
	}
	e.touch(t.InfoHash())
	return t, nil
}

func (e *Engine) touch(h metainfo.Hash) {
	e.mu.Lock()
	e.lastUsed[h] = time.Now()
	e.mu.Unlock()
}

func (e *Engine) dropTorrent(t *atorrent.Torrent) {
	name := ""
	if t.Info() != nil {
		name = t.Name()
	}
	h := t.InfoHash()
	t.Drop()
	e.mu.Lock()
	delete(e.lastUsed, h)
	e.mu.Unlock()
	// Reclaim disk: remove downloaded data for this torrent.
	if name != "" {
		_ = os.RemoveAll(filepath.Join(e.dataDir, name))
	}
}

// reaper evicts cached torrents: idle ones (not read within idleTTL) and, when
// the total cached bytes exceed maxBytes, the oldest ones until back under cap.
func (e *Engine) reaper() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		e.evictIdle()
		e.enforceDiskCap()
	}
}

func (e *Engine) evictIdle() {
	cutoff := time.Now().Add(-e.idleTTL)
	e.mu.Lock()
	stale := make([]metainfo.Hash, 0)
	for h, ts := range e.lastUsed {
		if ts.Before(cutoff) {
			stale = append(stale, h)
		}
	}
	e.mu.Unlock()
	for _, h := range stale {
		if t, ok := e.client.Torrent(h); ok {
			e.dropTorrent(t)
		}
	}
}

func (e *Engine) enforceDiskCap() {
	if e.maxBytes <= 0 {
		return
	}
	torrents := e.client.Torrents()
	var total int64
	for _, t := range torrents {
		total += t.BytesCompleted()
	}
	if total <= e.maxBytes {
		return
	}
	// Oldest-first by last-used (active torrents have the most recent timestamp
	// and are evicted last).
	e.mu.Lock()
	lastUsed := make(map[metainfo.Hash]time.Time, len(e.lastUsed))
	for h, ts := range e.lastUsed {
		lastUsed[h] = ts
	}
	e.mu.Unlock()
	sort.Slice(torrents, func(i, j int) bool {
		return lastUsed[torrents[i].InfoHash()].Before(lastUsed[torrents[j].InfoHash()])
	})
	for _, t := range torrents {
		if total <= e.maxBytes {
			break
		}
		total -= t.BytesCompleted()
		e.dropTorrent(t)
	}
}

var videoExt = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".mov": true, ".webm": true,
	".avi": true, ".wmv": true, ".ts": true, ".m2ts": true, ".mpg": true,
	".mpeg": true, ".flv": true,
}

func isVideo(name string) bool {
	return videoExt[strings.ToLower(filepath.Ext(name))]
}

var sxxeyy = regexp.MustCompile(`(?i)s(\d{1,2})[ ._-]*e(\d{1,3})`)

func matchEpisode(files []*atorrent.File, season, episode int) (*atorrent.File, int) {
	for i, f := range files {
		if !isVideo(f.DisplayPath()) {
			continue
		}
		m := sxxeyy.FindStringSubmatch(f.DisplayPath())
		if len(m) != 3 {
			continue
		}
		if atoiSafe(m[1]) == season && atoiSafe(m[2]) == episode {
			return f, i
		}
	}
	return nil, -1
}

// prioritizeEnds marks the first and last pieces of the chosen file as urgent so
// the player gets the header + index quickly (helps streaming start on low-seed torrents).
func prioritizeEnds(t *atorrent.Torrent, file *atorrent.File) {
	info := t.Info()
	if info == nil || info.PieceLength <= 0 {
		return
	}
	pl := info.PieceLength
	first := int(file.Offset() / pl)
	last := int((file.Offset() + file.Length() - 1) / pl)
	mark := func(from, to int) {
		for i := from; i <= to; i++ {
			if i >= 0 && i <= last {
				t.Piece(i).SetPriority(atorrent.PiecePriorityNow)
			}
		}
	}
	mark(first, first+4)
	if last-8 > first {
		mark(last-8, last)
	}
}

func chooseFile(t *atorrent.Torrent, fileIdx, season, episode int) (*atorrent.File, int) {
	files := t.Files()
	if len(files) == 0 {
		return nil, -1
	}
	if fileIdx >= 0 && fileIdx < len(files) {
		return files[fileIdx], fileIdx
	}
	if season > 0 && episode > 0 {
		if f, idx := matchEpisode(files, season, episode); f != nil {
			return f, idx
		}
	}
	bestIdx, bestSize := -1, int64(-1)
	for i, f := range files {
		if !isVideo(f.DisplayPath()) {
			continue
		}
		if f.Length() > bestSize {
			bestSize = f.Length()
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		return files[bestIdx], bestIdx
	}
	return nil, -1
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}
