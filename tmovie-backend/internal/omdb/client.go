package omdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type Detail struct {
	Title      string `json:"Title"`
	Year       string `json:"Year"`
	Rated      string `json:"Rated"`
	Released   string `json:"Released"`
	Runtime    string `json:"Runtime"`
	Genre      string `json:"Genre"`
	Director   string `json:"Director"`
	Writer     string `json:"Writer"`
	Actors     string `json:"Actors"`
	Plot       string `json:"Plot"`
	Language   string `json:"Language"`
	Country    string `json:"Country"`
	Awards     string `json:"Awards"`
	Poster     string `json:"Poster"`
	Ratings    []struct {
		Source string `json:"Source"`
		Value  string `json:"Value"`
	} `json:"Ratings"`
	Metascore  string `json:"Metascore"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	ImdbID     string `json:"imdbID"`
	Type       string `json:"Type"`
	Response   string `json:"Response"`
	Error      string `json:"Error"`
}

func (c *Client) ByIMDB(imdbID string) (*Detail, error) {
	if c.apiKey == "" || imdbID == "" {
		return nil, nil
	}
	q := url.Values{}
	q.Set("apikey", c.apiKey)
	q.Set("i", imdbID)
	q.Set("plot", "full")
	u := "http://www.omdbapi.com/?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("omdb %s", resp.Status)
	}
	var d Detail
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	if d.Response != "True" {
		if d.Error != "" {
			return nil, fmt.Errorf("omdb: %s", d.Error)
		}
		return nil, fmt.Errorf("omdb: not found")
	}
	return &d, nil
}
