package subdl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase = "https://api.subdl.com/api/v1/subtitles"
	dlBase  = "https://dl.subdl.com"
)

// Subtitle is one SubDL result, normalized.
type Subtitle struct {
	Lang        string // ISO-ish, lowercased (e.g. "en")
	Language    string // SubDL language label (e.g. "English")
	ZipPath     string // path under dl.subdl.com, e.g. /subtitle/123.zip
	ReleaseName string
	Name        string
}

// Client talks to the SubDL public API.
type Client struct {
	apiKey string
	langs  string
	http   *http.Client
}

func New(apiKey, languages string) *Client {
	langs := strings.TrimSpace(languages)
	if langs == "" {
		langs = "EN"
	}
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		langs:  langs,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// isoFromName maps a SubDL language name to a 2-letter ISO code (fallback path).
func isoFromName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "english":
		return "en"
	case "croatian":
		return "hr"
	case "serbian":
		return "sr"
	case "bosnian":
		return "bs"
	case "slovenian", "slovene":
		return "sl"
	case "macedonian":
		return "mk"
	case "spanish":
		return "es"
	case "german":
		return "de"
	case "french":
		return "fr"
	case "italian":
		return "it"
	default:
		n := strings.ToLower(strings.TrimSpace(name))
		if len(n) >= 2 {
			return n[:2]
		}
		return n
	}
}

// DownloadBase is the host prefix for ZipPath.
func DownloadBase() string { return dlBase }

type apiResponse struct {
	Status    bool `json:"status"`
	Subtitles []struct {
		ReleaseName string `json:"release_name"`
		Name        string `json:"name"`
		Lang        string `json:"lang"`
		Language    string `json:"language"`
		URL         string `json:"url"`
		Season      int    `json:"season"`
		Episode     int    `json:"episode"`
	} `json:"subtitles"`
}

// Search queries SubDL by IMDb id. For series, pass season/episode (>0).
func (c *Client) Search(imdbID string, season, episode int) ([]Subtitle, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("subdl api key not set")
	}
	imdb := strings.TrimSpace(imdbID)
	if imdb == "" {
		return nil, fmt.Errorf("missing imdb id")
	}
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("imdb_id", imdb)
	q.Set("languages", c.langs)
	q.Set("subs_per_page", "30")
	if season > 0 && episode > 0 {
		q.Set("type", "tv")
		q.Set("season_number", strconv.Itoa(season))
		q.Set("episode_number", strconv.Itoa(episode))
	} else {
		q.Set("type", "movie")
	}
	resp, err := c.http.Get(apiBase + "?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subdl http %d", resp.StatusCode)
	}
	var parsed apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]Subtitle, 0, len(parsed.Subtitles))
	for _, s := range parsed.Subtitles {
		if strings.TrimSpace(s.URL) == "" {
			continue
		}
		// For series, keep only the requested episode when SubDL tags it.
		if season > 0 && episode > 0 && s.Season > 0 && s.Episode > 0 {
			if s.Season != season || s.Episode != episode {
				continue
			}
		}
		// SubDL: `language` is the 2-letter ISO code (e.g. "EN"); `lang` is the
		// full name (e.g. "English"). The app filters by ISO code, so prefer it.
		lang := strings.ToLower(strings.TrimSpace(s.Language))
		if lang == "" || len(lang) > 3 {
			lang = isoFromName(s.Lang)
		}
		out = append(out, Subtitle{
			Lang:        lang,
			Language:    s.Lang,
			ZipPath:     s.URL,
			ReleaseName: s.ReleaseName,
			Name:        s.Name,
		})
	}
	return out, nil
}
