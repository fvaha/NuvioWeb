package handlers

import (
	"net/http"

	"tmovie/internal/catalog"
	"tmovie/internal/images"

	"github.com/gin-gonic/gin"
)

func ifaceString(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func youTubeMuteEmbed(key string) string {
	if key == "" {
		return ""
	}
	// loop+playlist repeats the same short; autoplay mute for TV storefront-style preview.
	return "https://www.youtube-nocookie.com/embed/" + key +
		"?autoplay=1&mute=1&controls=0&playsinline=1&rel=0&modestbranding=1" +
		"&loop=1&playlist=" + key
}

// browseHomeHero returns one trending movie or TV row that has an official TMDB YouTube trailer (for muted home autoplay).
func (a *API) browseHomeHero(c *gin.Context) {
	const maxScan = 12
	empty := gin.H{
		"youtube_key":   "",
		"embed_url":     "",
		"title":         "",
		"overview":      "",
		"poster_url":    "",
		"backdrop_url":  "",
		"tmdb_id":       0,
		"media_type":    "",
		"display_title": "",
	}

	movies, err := a.TMDB.TrendingMovies(1)
	if err == nil {
		for i, row := range movies {
			if i >= maxScan {
				break
			}
			m, err := a.TMDB.MovieAppend(row.ID)
			if err != nil {
				continue
			}
			key := catalog.FirstTrailerYouTubeKey(m)
			if key == "" {
				continue
			}
			backdrop := images.TMDB(ifaceString(m["backdrop_path"]), "w1280")
			if backdrop == "" {
				backdrop = images.TMDB(row.PosterPath, "w1280")
			}
			c.JSON(http.StatusOK, gin.H{
				"youtube_key":   key,
				"embed_url":     youTubeMuteEmbed(key),
				"display_title": row.Title,
				"title":         row.Title,
				"overview":      row.Overview,
				"poster_url":    images.TMDB(row.PosterPath, "w780"),
				"backdrop_url":  backdrop,
				"tmdb_id":       row.ID,
				"media_type":    "movie",
			})
			return
		}
	}

	tvRows, err := a.TMDB.TrendingTV(1)
	if err == nil {
		for i, row := range tvRows {
			if i >= maxScan {
				break
			}
			tvm, err := a.TMDB.TVAppend(row.ID)
			if err != nil {
				continue
			}
			key := catalog.FirstTrailerYouTubeKey(tvm)
			if key == "" {
				continue
			}
			backdrop := images.TMDB(ifaceString(tvm["backdrop_path"]), "w1280")
			if backdrop == "" {
				backdrop = images.TMDB(row.PosterPath, "w1280")
			}
			c.JSON(http.StatusOK, gin.H{
				"youtube_key":   key,
				"embed_url":     youTubeMuteEmbed(key),
				"display_title": row.Name,
				"title":         row.Name,
				"overview":      row.Overview,
				"poster_url":    images.TMDB(row.PosterPath, "w780"),
				"backdrop_url":  backdrop,
				"tmdb_id":       row.ID,
				"media_type":    "tv",
			})
			return
		}
	}

	c.JSON(http.StatusOK, empty)
}
