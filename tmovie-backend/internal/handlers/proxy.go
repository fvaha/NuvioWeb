package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type ScraperResponse struct {
	Stream struct {
		Playlist string `json:"playlist"`
		Type     string `json:"type"` // "hls", "mp4"
		URL      string `json:"url"` // sometimes direct url instead of playlist
	} `json:"stream"`
	SourceId string `json:"sourceId"`
	Error    string `json:"error"`
}

func (a *API) proxyInit(c *gin.Context) {
	tmdbID := c.Query("tmdb_id")
	imdbID := c.Query("imdb_id")
	mediaType := c.Query("media_type")
	season := c.Query("season")
	episode := c.Query("episode")
	provider := c.Query("provider")

	if tmdbID == "" || mediaType == "" {
		c.String(http.StatusBadRequest, "Missing tmdb_id or media_type")
		return
	}

	if tmdbID == "999" {
		c.JSON(http.StatusOK, gin.H{"url": "/api/v1/proxy/m3u8/play.m3u8?url=" + url.QueryEscape("https://devstreaming-cdn.apple.com/videos/streaming/examples/img_bipbop_adv_example_ts/master.m3u8")})
		return
	}

	scraperURL := fmt.Sprintf("http://127.0.0.1:3000/extract?tmdbId=%s&mediaType=%s", tmdbID, mediaType)
	if imdbID != "" {
		scraperURL += "&imdbId=" + imdbID
	}
	if provider != "" {
		scraperURL += "&source=" + provider
	}
	if mediaType == "tv" {
		scraperURL += fmt.Sprintf("&season=%s&episode=%s", season, episode)
	}

	resp, err := http.Get(scraperURL)
	if err != nil {
		c.String(http.StatusBadGateway, "Failed to contact scraper: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.String(http.StatusBadGateway, "Scraper error: "+string(body))
		return
	}

	var data ScraperResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.String(http.StatusInternalServerError, "Invalid scraper response")
		return
	}

	streamURL := data.Stream.Playlist
	if streamURL == "" {
		streamURL = data.Stream.URL
	}

	if streamURL == "" {
		c.String(http.StatusNotFound, "No stream found")
		return
	}

	if data.Stream.Type == "mp4" {
		c.JSON(http.StatusOK, gin.H{"url": streamURL})
		return
	}

	hostOrigin := publicOrigin(c, a.Cfg)
	proxyURL := hostOrigin + "/api/v1/proxy/m3u8/play.m3u8?url=" + url.QueryEscape(streamURL)
	c.JSON(http.StatusOK, gin.H{"url": proxyURL})
}

func (a *API) proxyM3U8(c *gin.Context) {
	streamURL := c.Query("url")
	if streamURL == "" {
		c.String(http.StatusBadRequest, "Missing url")
		return
	}
	a.serveM3U8(c, streamURL)
}

func (a *API) serveM3U8(c *gin.Context, streamURL string) {
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Invalid stream url")
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", streamURL)
	req.Header.Set("Origin", "https://vidsrc.to") // Keep just in case some servers expect it

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "Failed to fetch m3u8: "+err.Error())
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Access-Control-Allow-Origin", "*")

	base, err := url.Parse(streamURL)
	if err != nil {
		io.Copy(c.Writer, resp.Body)
		return
	}

	hostOrigin := publicOrigin(c, a.Cfg)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			c.Writer.Write([]byte(line + "\n"))
			continue
		}

		// Handle tags with URIs like #EXT-X-MEDIA:...,URI="path"
		if strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, "URI=\"") {
				parts := strings.Split(trimmed, "URI=\"")
				if len(parts) > 1 {
					subParts := strings.SplitN(parts[1], "\"", 2)
					if len(subParts) > 1 {
						originalURI := subParts[0]
						targetURL := originalURI
						if !strings.HasPrefix(targetURL, "http") {
							rel, err := url.Parse(targetURL)
							if err == nil {
								targetURL = base.ResolveReference(rel).String()
							}
						}
						
						var proxyURL string
						if strings.HasSuffix(strings.Split(targetURL, "?")[0], ".m3u8") {
							proxyURL = hostOrigin + "/api/v1/proxy/m3u8/play.m3u8?url=" + url.QueryEscape(targetURL)
						} else {
							proxyURL = hostOrigin + "/api/v1/proxy/ts/segment.ts?url=" + url.QueryEscape(targetURL)
						}
						
						line = parts[0] + "URI=\"" + proxyURL + "\"" + subParts[1]
					}
				}
			}
			c.Writer.Write([]byte(line + "\n"))
			continue
		}

		targetURL := trimmed
		if !strings.HasPrefix(targetURL, "http") {
			rel, err := url.Parse(targetURL)
			if err == nil {
				targetURL = base.ResolveReference(rel).String()
			}
		}

		var proxyURL string
		if strings.HasSuffix(strings.Split(targetURL, "?")[0], ".m3u8") {
			proxyURL = hostOrigin + "/api/v1/proxy/m3u8/play.m3u8?url=" + url.QueryEscape(targetURL)
		} else {
			proxyURL = hostOrigin + "/api/v1/proxy/ts/segment.ts?url=" + url.QueryEscape(targetURL)
		}

		c.Writer.Write([]byte(proxyURL + "\n"))
	}
}

func (a *API) proxyTS(c *gin.Context) {
	tsURL := c.Query("url")
	if tsURL == "" {
		c.String(http.StatusBadRequest, "Missing url")
		return
	}

	req, err := http.NewRequest("GET", tsURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Invalid ts url")
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", tsURL)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "Failed to fetch ts")
		return
	}
	defer resp.Body.Close()

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}

	io.Copy(c.Writer, resp.Body)
}
