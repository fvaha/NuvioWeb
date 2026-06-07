package sources

import (
	"fmt"
	"sort"
	"strings"
)

type SourceKind string

const (
	SourceIframe SourceKind = "iframe"
	// SourceDirect = same-origin file URL; Tizen app uses Samsung AVPlay (not iframe).
	SourceDirect SourceKind = "direct"
)

// PlaySource is one playable embed; lower Priority = listed first (preferred default).
type PlaySource struct {
	Name     string     `json:"name"`
	URL      string     `json:"url"`
	Kind     SourceKind `json:"kind"`
	Priority int        `json:"priority"`
	Provider string     `json:"provider,omitempty"`
}

func (s PlaySource) valid() bool {
	return s.URL != "" && (strings.HasPrefix(s.URL, "http") || strings.HasPrefix(s.URL, "/"))
}

func dedupeSort(src []PlaySource) []PlaySource {
	seen := map[string]struct{}{}
	var out []PlaySource
	for _, p := range src {
		if !p.valid() {
			continue
		}
		if _, ok := seen[p.URL]; ok {
			continue
		}
		seen[p.URL] = struct{}{}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// MovieEmbedURLs aggregates known TMDB-based embed players.
func MovieEmbedURLs(tmdbID int, imdbID string) []PlaySource {
	imdbNum := strings.TrimPrefix(strings.TrimSpace(imdbID), "tt")
	var all []PlaySource
	
	// Define Native Proxy sources
	providers := []string{"vidsrc", "vidcloud", "superembed", "flixtor"}
	for i, p := range providers {
		u := fmt.Sprintf("/api/v1/proxy/init?tmdb_id=%d&media_type=movie&provider=%s", tmdbID, p)
		if imdbNum != "" {
			u += fmt.Sprintf("&imdb_id=tt%s", imdbNum)
		}
		all = append(all, PlaySource{
			Name:     fmt.Sprintf("Native Proxy (%s)", strings.Title(p)),
			Provider: "proxy",
			Priority: 5 + i,
			Kind:     SourceDirect,
			URL:      u,
		})
	}

	return dedupeSort(all)
}

// TVEmbedURLs builds TV episode embeds.
func TVEmbedURLs(tmdbTVID, season, episode int) []PlaySource {
	var all []PlaySource

	// Define Native Proxy sources
	providers := []string{"vidsrc", "vidcloud", "superembed", "flixtor"}
	for i, p := range providers {
		u := fmt.Sprintf("/api/v1/proxy/init?tmdb_id=%d&media_type=tv&season=%d&episode=%d&provider=%s", tmdbTVID, season, episode, p)
		all = append(all, PlaySource{
			Name:     fmt.Sprintf("Native Proxy (%s)", strings.Title(p)),
			Provider: "proxy",
			Priority: 5 + i,
			Kind:     SourceDirect,
			URL:      u,
		})
	}

	return dedupeSort(all)
}
