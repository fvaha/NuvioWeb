package tmdb

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) MovieAppend(id int) (map[string]interface{}, error) {
	q := url.Values{}
	q.Set("append_to_response", "credits,external_ids,images,videos,keywords,recommendations,reviews")
	body, err := c.get("/movie/"+strconv.Itoa(id), q)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Client) TVAppend(id int) (map[string]interface{}, error) {
	q := url.Values{}
	q.Set("append_to_response", "credits,external_ids,images,videos,keywords,recommendations,reviews,aggregate_credits")
	body, err := c.get("/tv/"+strconv.Itoa(id), q)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Client) TVEpisode(tvID, season, episode int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/tv/%d/season/%d/episode/%d", tvID, season, episode)
	q := url.Values{}
	q.Set("append_to_response", "credits,external_ids,images,guest_stars")
	body, err := c.get(path, q)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}
