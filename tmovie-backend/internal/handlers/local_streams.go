package handlers

import (
	"net/http"
	"strconv"

	"tmovie/internal/config"
	"tmovie/internal/sources"

	"github.com/gin-gonic/gin"
)

func publicOrigin(c *gin.Context, cfg config.Config) string {
	if b := cfg.PublicBaseURL; b != "" {
		return trimSlash(b)
	}
	if c.Request == nil || c.Request.Host == "" {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func mergeSourcesFirst(payload map[string]interface{}, first sources.PlaySource) {
	exist, ok := payload["sources"].([]sources.PlaySource)
	if !ok {
		return
	}
	out := make([]sources.PlaySource, 0, len(exist)+1)
	out = append(out, first)
	out = append(out, exist...)
	payload["sources"] = out
}

func prependLocalSubtitleTrack(payload map[string]interface{}, fileURL string) {
	subs, ok := payload["subtitles"].(map[string]interface{})
	if !ok {
		return
	}
	raw := subs["tracks"]
	arr, ok := raw.([]map[string]interface{})
	if !ok {
		return
	}
	local := map[string]interface{}{
		"file_id":   0,
		"file_name": "local.srt",
		"language":  "local",
		"release":   "disk",
		"pull_url":  "",
		"file_url":  fileURL,
	}
	out := make([]map[string]interface{}, 0, len(arr)+1)
	out = append(out, local)
	out = append(out, arr...)
	subs["tracks"] = out
	payload["subtitles"] = subs
}

func (a *API) enrichMovieLocal(c *gin.Context, payload map[string]interface{}, id int) {
	if a.Local == nil || !a.Local.Enabled() {
		return
	}
	origin := publicOrigin(c, a.Cfg)
	if origin == "" {
		return
	}
	if src, ok := a.Local.MoviePlaySource(origin, id); ok {
		mergeSourcesFirst(payload, src)
	}
	if a.Local.HasMovieSubtitle(id) {
		u := origin + "/api/v1/local/subs/movie/" + strconv.Itoa(id)
		prependLocalSubtitleTrack(payload, u)
	}
}

func (a *API) enrichTVLocal(c *gin.Context, payload map[string]interface{}, showID, season, episode int) {
	if a.Local == nil || !a.Local.Enabled() {
		return
	}
	origin := publicOrigin(c, a.Cfg)
	if origin == "" {
		return
	}
	if src, ok := a.Local.TVPlaySource(origin, showID, season, episode); ok {
		mergeSourcesFirst(payload, src)
	}
	if a.Local.HasTVSubtitle(showID, season, episode) {
		u := origin + "/api/v1/local/subs/tv/" + strconv.Itoa(showID) + "/" + strconv.Itoa(season) + "/" + strconv.Itoa(episode)
		prependLocalSubtitleTrack(payload, u)
	}
}

func (a *API) localStreamMovie(c *gin.Context) {
	if a.Local == nil || !a.Local.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "local library not configured"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	abs, ok := a.Local.MovieAbsPath(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local file mapped for this movie"})
		return
	}
	a.Local.ServeVideo(c.Writer, c.Request, abs)
}

func (a *API) localStreamTV(c *gin.Context) {
	if a.Local == nil || !a.Local.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "local library not configured"})
		return
	}
	showID, err := strconv.Atoi(c.Param("id"))
	if err != nil || showID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid show id"})
		return
	}
	season, err := strconv.Atoi(c.Param("season"))
	if err != nil || season < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid season"})
		return
	}
	episode, err := strconv.Atoi(c.Param("episode"))
	if err != nil || episode < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid episode"})
		return
	}
	abs, ok := a.Local.TVAbsPath(showID, season, episode)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local file mapped for this episode"})
		return
	}
	a.Local.ServeVideo(c.Writer, c.Request, abs)
}

func (a *API) localSubsMovie(c *gin.Context) {
	if a.Local == nil || !a.Local.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "local library not configured"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	video, ok := a.Local.MovieAbsPath(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local video"})
		return
	}
	srt, ok := a.Local.FirstSRTBesideVideo(video)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local srt"})
		return
	}
	a.Local.ServeSubtitle(c.Writer, c.Request, srt)
}

func (a *API) localSubsTV(c *gin.Context) {
	if a.Local == nil || !a.Local.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "local library not configured"})
		return
	}
	showID, err := strconv.Atoi(c.Param("id"))
	if err != nil || showID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid show id"})
		return
	}
	season, err := strconv.Atoi(c.Param("season"))
	if err != nil || season < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid season"})
		return
	}
	episode, err := strconv.Atoi(c.Param("episode"))
	if err != nil || episode < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid episode"})
		return
	}
	video, ok := a.Local.TVAbsPath(showID, season, episode)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local video"})
		return
	}
	srt, ok := a.Local.FirstSRTBesideVideo(video)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no local srt"})
		return
	}
	a.Local.ServeSubtitle(c.Writer, c.Request, srt)
}
