package tmdb

import (
	"encoding/json"
	"net/url"
	"strconv"
)

type MovieListItem struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	ReleaseDate   string  `json:"release_date"`
	OriginalTitle string  `json:"original_title"`
}

type TVListItem struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	FirstAirDate  string  `json:"first_air_date"`
	OriginalName  string  `json:"original_name"`
}

func (c *Client) TrendingMovies(page int) ([]MovieListItem, error) {
	return c.movieList("/trending/movie/week", page)
}

func (c *Client) TrendingTV(page int) ([]TVListItem, error) {
	return c.tvList("/trending/tv/week", page)
}

func (c *Client) PopularMovies(page int) ([]MovieListItem, error) {
	return c.movieList("/movie/popular", page)
}

func (c *Client) PopularTV(page int) ([]TVListItem, error) {
	return c.tvList("/tv/popular", page)
}

func (c *Client) movieList(path string, page int) ([]MovieListItem, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	body, err := c.get(path, q)
	if err != nil {
		return nil, err
	}
	var out struct {
		Results []MovieListItem `json:"results"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func (c *Client) tvList(path string, page int) ([]TVListItem, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	body, err := c.get(path, q)
	if err != nil {
		return nil, err
	}
	var out struct {
		Results []TVListItem `json:"results"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}
