package subtitles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://api.opensubtitles.com/api/v1"

type Client struct {
	apiKey   string
	username string
	password string
	http     *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func New(apiKey, username, password string) *Client {
	return &Client{
		apiKey:   strings.TrimSpace(apiKey),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != "" && c.username != "" && c.password != ""
}

func (c *Client) authHeaders(req *http.Request) error {
	if !c.Enabled() {
		return fmt.Errorf("opensubtitles not configured")
	}
	tok, err := c.bearer()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", tok)
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tmovie-backend/1.0")
	return nil
}

// searchHeaders sets only what the search endpoint needs (no login).
func (c *Client) searchHeaders(req *http.Request) {
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tmovie-backend/1.0")
}

func (c *Client) bearer() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-2*time.Minute)) {
		return "Bearer " + c.token, nil
	}
	body, err := json.Marshal(map[string]string{
		"username": c.username,
		"password": c.password,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tmovie-backend/1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("opensubtitles login %s: %s", resp.Status, truncate(raw, 300))
	}
	var envelope struct {
		Token string `json:"token"`
		Data  struct {
			Token string `json:"token"`
		} `json:"data"`
		User struct {
			Token string `json:"token"`
		} `json:"user"`
	}
	_ = json.Unmarshal(raw, &envelope)
	tok := envelope.Token
	if tok == "" {
		tok = envelope.Data.Token
	}
	if tok == "" {
		tok = envelope.User.Token
	}
	if tok == "" {
		return "", fmt.Errorf("opensubtitles login: empty token (%s)", truncate(raw, 200))
	}
	c.token = tok
	c.tokenExpiry = time.Now().Add(12 * time.Hour)
	return "Bearer " + c.token, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

type Track struct {
	FileID      int64  `json:"file_id"`
	FileName    string `json:"file_name"`
	Language    string `json:"language"`
	Release     string `json:"release"`
	DownloadURL string `json:"download_url"`
}

func (c *Client) SearchMovie(imdbNumeric string, languages string) ([]Track, error) {
	if !c.Enabled() {
		return nil, nil
	}
	q := url.Values{}
	q.Set("imdb_id", strings.TrimPrefix(imdbNumeric, "tt"))
	if languages != "" {
		q.Set("languages", languages)
	}
	return c.search(q)
}

func (c *Client) SearchEpisode(parentImdbNumeric string, season, episode int, languages string) ([]Track, error) {
	if !c.Enabled() {
		return nil, nil
	}
	q := url.Values{}
	q.Set("parent_imdb_id", strings.TrimPrefix(parentImdbNumeric, "tt"))
	q.Set("season_number", strconv.Itoa(season))
	q.Set("episode_number", strconv.Itoa(episode))
	if languages != "" {
		q.Set("languages", languages)
	}
	return c.search(q)
}

func (c *Client) SearchTVEpisode(tmdbShowID, season, episode int, languages string) ([]Track, error) {
	if !c.Enabled() {
		return nil, nil
	}
	q := url.Values{}
	q.Set("tmdb_id", strconv.Itoa(tmdbShowID))
	q.Set("season_number", strconv.Itoa(season))
	q.Set("episode_number", strconv.Itoa(episode))
	if languages != "" {
		q.Set("languages", languages)
	}
	return c.search(q)
}

func (c *Client) search(q url.Values) ([]Track, error) {
	u, err := url.Parse(apiBase + "/subtitles")
	if err != nil {
		return nil, err
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// Search only needs the Api-Key (no login). Avoid bearer() so a failed/expired
	// login can't break search.
	c.searchHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("opensubtitles search %s: %s", resp.Status, truncate(raw, 300))
	}
	var envelope struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Release       string `json:"release"`
				Language      string `json:"language"`
				SubtitleID    string `json:"subtitle_id"`
				Files         []struct {
					FileID   int64  `json:"file_id"`
					FileName string `json:"file_name"`
				} `json:"files"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	var tracks []Track
	for _, row := range envelope.Data {
		for _, f := range row.Attributes.Files {
			if f.FileID == 0 {
				continue
			}
			tracks = append(tracks, Track{
				FileID:   f.FileID,
				FileName: f.FileName,
				Language: row.Attributes.Language,
				Release:  row.Attributes.Release,
			})
		}
	}
	return tracks, nil
}

type DownloadResult struct {
	LocalRelPath string
	Bytes        int64
}

func (c *Client) DownloadToDisk(fileID int64, destDir, destName string) (*DownloadResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("opensubtitles not configured")
	}
	body, err := json.Marshal(map[string]any{"file_id": fileID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+"/download", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Download works with the Api-Key alone; avoid login (bearer) so stale/invalid
	// username/password can't break it.
	c.searchHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("opensubtitles download meta %s: %s", resp.Status, truncate(raw, 300))
	}
	var dl struct {
		Link     string `json:"link"`
		FileName string `json:"file_name"`
		Data     struct {
			Link     string `json:"link"`
			FileName string `json:"file_name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &dl)
	link := dl.Link
	if link == "" {
		link = dl.Data.Link
	}
	if link == "" {
		return nil, fmt.Errorf("opensubtitles download: missing link (%s)", truncate(raw, 400))
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	outName := destName
	if outName == "" {
		outName = dl.FileName
		if outName == "" {
			outName = dl.Data.FileName
		}
		if outName == "" {
			outName = fmt.Sprintf("subtitle_%d.srt", fileID)
		}
	}
	outName = filepath.Base(outName)
	destPath := filepath.Join(destDir, outName)
	getReq, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	getReq.Header.Set("User-Agent", "tmovie-backend/1.0")
	dlr, err := c.http.Do(getReq)
	if err != nil {
		return nil, err
	}
	defer dlr.Body.Close()
	if dlr.StatusCode < 200 || dlr.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(dlr.Body, 512))
		return nil, fmt.Errorf("opensubtitles file fetch %s: %s", dlr.Status, string(b))
	}
	f, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := io.Copy(f, dlr.Body)
	if err != nil {
		return nil, err
	}
	rel := filepath.ToSlash(filepath.Join("subtitles", outName))
	return &DownloadResult{LocalRelPath: rel, Bytes: n}, nil
}
