package handlers

import (
	"net/http"
	"strconv"

	"tmovie/internal/images"

	"github.com/gin-gonic/gin"
)

func (a *API) tvSeasonsList(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid show id"})
		return
	}
	name, seasons, err := a.TMDB.TVSeasonsOutline(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(seasons))
	for _, s := range seasons {
		pu := ""
		if s.PosterPath != "" {
			pu = images.TMDB(s.PosterPath, "w185")
		}
		out = append(out, gin.H{
			"season_number": s.SeasonNumber,
			"episode_count": s.EpisodeCount,
			"name":          s.Name,
			"air_date":      s.AirDate,
			"poster_url":    pu,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"show_id": id,
		"name":    name,
		"seasons": out,
	})
}

func (a *API) tvSeasonEpisodesList(c *gin.Context) {
	showID, err := strconv.Atoi(c.Param("id"))
	if err != nil || showID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid show id"})
		return
	}
	seasonNum, err := strconv.Atoi(c.Param("season"))
	if err != nil || seasonNum < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid season"})
		return
	}
	seasonNo, seasonAir, eps, err := a.TMDB.TVSeasonEpisodes(showID, seasonNum)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	rows := make([]gin.H, 0, len(eps))
	for _, e := range eps {
		su := ""
		if e.StillPath != "" {
			su = images.TMDB(e.StillPath, "w300")
		}
		rows = append(rows, gin.H{
			"episode_number": e.EpisodeNumber,
			"name":           e.Name,
			"air_date":       e.AirDate,
			"overview":       e.Overview,
			"still_url":      su,
			"runtime":        e.Runtime,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"show_id":        showID,
		"season_number":  seasonNo,
		"season_air_date": seasonAir,
		"episodes":       rows,
	})
}
