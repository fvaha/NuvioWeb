package tmdb

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type TVSeasonSummary struct {
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int  `json:"episode_count"`
	Name         string `json:"name"`
	AirDate      string `json:"air_date,omitempty"`
	PosterPath   string `json:"poster_path"`
}

// TVSeasonsOutline is name + per-season counts from GET /tv/{id}.
func (c *Client) TVSeasonsOutline(showID int) (showName string, seasons []TVSeasonSummary, err error) {
	body, err := c.get("/tv/"+strconv.Itoa(showID), nil)
	if err != nil {
		return "", nil, err
	}
	var raw struct {
		Name    string `json:"name"`
		Seasons []struct {
			AirDate       *string `json:"air_date"`
			EpisodeCount  int     `json:"episode_count"`
			Name          string  `json:"name"`
			SeasonNumber  int     `json:"season_number"`
			PosterPath    string  `json:"poster_path"`
		} `json:"seasons"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", nil, err
	}
	out := make([]TVSeasonSummary, 0, len(raw.Seasons))
	for _, s := range raw.Seasons {
		if s.EpisodeCount < 1 {
			continue
		}
		ad := ""
		if s.AirDate != nil {
			ad = *s.AirDate
		}
		out = append(out, TVSeasonSummary{
			SeasonNumber: s.SeasonNumber,
			EpisodeCount: s.EpisodeCount,
			Name:         s.Name,
			AirDate:      ad,
			PosterPath:   s.PosterPath,
		})
	}
	return raw.Name, out, nil
}

// TVEpisodeRow is one episode from GET /tv/{id}/season/{n}.
type TVEpisodeRow struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	AirDate       string `json:"air_date"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	StillURL      string `json:"still_url,omitempty"`
	Runtime       int    `json:"runtime"`
}

func (c *Client) TVSeasonEpisodes(showID, seasonNumber int) (seasonNum int, airDate string, eps []TVEpisodeRow, err error) {
	path := fmt.Sprintf("/tv/%d/season/%d", showID, seasonNumber)
	body, err := c.get(path, nil)
	if err != nil {
		return 0, "", nil, err
	}
	var raw struct {
		AirDate       string `json:"air_date"`
		SeasonNumber  int    `json:"season_number"`
		Episodes      []struct {
			AirDate       string `json:"air_date"`
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			Overview      string `json:"overview"`
			StillPath     string `json:"still_path"`
			Runtime       int    `json:"runtime"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, "", nil, err
	}
	rows := make([]TVEpisodeRow, 0, len(raw.Episodes))
	for _, e := range raw.Episodes {
		rows = append(rows, TVEpisodeRow{
			EpisodeNumber: e.EpisodeNumber,
			Name:          e.Name,
			AirDate:       e.AirDate,
			Overview:      e.Overview,
			StillPath:     e.StillPath,
			Runtime:       e.Runtime,
		})
	}
	return raw.SeasonNumber, raw.AirDate, rows, nil
}
