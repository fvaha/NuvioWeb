package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) get(path string, q url.Values) ([]byte, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY is not set")
	}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("api_key", c.apiKey)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tmdb %s: %s", resp.Status, truncate(body, 200))
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// MultiSearchResult is one row from /search/multi (movies use title, tv uses name).
type MultiSearchResult struct {
	ID            int    `json:"id"`
	MediaType     string `json:"media_type"`
	Title         string `json:"title"`
	Name          string `json:"name"`
	PosterPath    string `json:"poster_path"`
	Overview      string `json:"overview"`
	ReleaseDate   string `json:"release_date"`
	FirstAirDate  string `json:"first_air_date"`
	OriginalTitle string `json:"original_title"`
}

func (r MultiSearchResult) DisplayTitle() string {
	switch r.MediaType {
	case "tv":
		if r.Name != "" {
			return r.Name
		}
	case "movie":
		if r.Title != "" {
			return r.Title
		}
	}
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

func (c *Client) SearchMulti(query string, page int) ([]MultiSearchResult, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("page", strconv.Itoa(page))
	body, err := c.get("/search/multi", q)
	if err != nil {
		return nil, err
	}
	var out struct {
		Results []MultiSearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

type MovieDetail struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	PosterPath   string `json:"poster_path"`
	Overview     string `json:"overview"`
	ReleaseDate  string `json:"release_date"`
	Runtime      int    `json:"runtime"`
	OriginalTitle string `json:"original_title"`
}

func (c *Client) MovieDetail(id int) (*MovieDetail, error) {
	body, err := c.get("/movie/"+strconv.Itoa(id), nil)
	if err != nil {
		return nil, err
	}
	var d MovieDetail
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

type TVDetail struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	PosterPath    string `json:"poster_path"`
	Overview      string `json:"overview"`
	FirstAirDate  string `json:"first_air_date"`
	EpisodeRunTime []int `json:"episode_run_time"`
}

func (c *Client) TVDetail(id int) (*TVDetail, error) {
	body, err := c.get("/tv/"+strconv.Itoa(id), nil)
	if err != nil {
		return nil, err
	}
	var d TVDetail
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
