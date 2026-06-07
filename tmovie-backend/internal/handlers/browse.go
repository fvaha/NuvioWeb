package handlers

import (
	"net/http"

	"tmovie/internal/config"
	"tmovie/internal/images"
	"tmovie/internal/tmdb"

	"github.com/gin-gonic/gin"
)

func (a *API) browseTrendingMovies(c *gin.Context) {
	page := config.Atoi(c.DefaultQuery("page", "1"), 1)
	rows, err := a.TMDB.TrendingMovies(page)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "section": "trending_movies", "results": formatMovieBrowse(rows)})
}

func (a *API) browseTrendingTV(c *gin.Context) {
	page := config.Atoi(c.DefaultQuery("page", "1"), 1)
	rows, err := a.TMDB.TrendingTV(page)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "section": "trending_tv", "results": formatTVBrowse(rows)})
}

func (a *API) browsePopularMovies(c *gin.Context) {
	page := config.Atoi(c.DefaultQuery("page", "1"), 1)
	rows, err := a.TMDB.PopularMovies(page)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "section": "popular_movies", "results": formatMovieBrowse(rows)})
}

func (a *API) browsePopularTV(c *gin.Context) {
	page := config.Atoi(c.DefaultQuery("page", "1"), 1)
	rows, err := a.TMDB.PopularTV(page)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "section": "popular_tv", "results": formatTVBrowse(rows)})
}

func formatMovieBrowse(rows []tmdb.MovieListItem) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":                  r.ID,
			"media_type":          "movie",
			"title":               r.Title,
			"display_title":       r.Title,
			"overview":            r.Overview,
			"poster_path":         r.PosterPath,
			"poster_url":          images.TMDB(r.PosterPath, "w342"),
			"vote_average":        r.VoteAverage,
			"release_or_air_date": r.ReleaseDate,
		})
	}
	return out
}

func formatTVBrowse(rows []tmdb.TVListItem) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":                  r.ID,
			"media_type":          "tv",
			"name":                r.Name,
			"display_title":       r.Name,
			"overview":            r.Overview,
			"poster_path":         r.PosterPath,
			"poster_url":          images.TMDB(r.PosterPath, "w342"),
			"vote_average":        r.VoteAverage,
			"release_or_air_date": r.FirstAirDate,
		})
	}
	return out
}
