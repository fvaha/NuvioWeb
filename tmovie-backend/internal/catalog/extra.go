package catalog

import (
	"net/url"
	"strconv"

	"tmovie/internal/images"
	"tmovie/internal/omdb"
	"tmovie/internal/subtitles"
)

func firstBackdropFile(m map[string]interface{}) string {
	img := asMap(m["images"])
	if img == nil {
		return ""
	}
	backs := asSlice(img["backdrops"])
	if len(backs) == 0 {
		return ""
	}
	mm := asMap(backs[0])
	return str(mm, "file_path")
}

func posterFiles(m map[string]interface{}, n int) []string {
	img := asMap(m["images"])
	if img == nil {
		return nil
	}
	items := asSlice(img["posters"])
	out := []string{}
	for i, it := range items {
		if i >= n {
			break
		}
		mm := asMap(it)
		if mm == nil {
			continue
		}
		fp := str(mm, "file_path")
		if fp == "" {
			continue
		}
		out = append(out, images.TMDB(fp, "w780"))
	}
	return out
}

func backdropFiles(m map[string]interface{}, n int) []string {
	img := asMap(m["images"])
	if img == nil {
		return nil
	}
	items := asSlice(img["backdrops"])
	out := []string{}
	for i, it := range items {
		if i >= n {
			break
		}
		mm := asMap(it)
		if mm == nil {
			continue
		}
		fp := str(mm, "file_path")
		if fp == "" {
			continue
		}
		out = append(out, images.TMDB(fp, "w1280"))
	}
	return out
}

func logoFiles(m map[string]interface{}, n int) []string {
	img := asMap(m["images"])
	if img == nil {
		return nil
	}
	items := asSlice(img["logos"])
	out := []string{}
	for i, it := range items {
		if i >= n {
			break
		}
		mm := asMap(it)
		if mm == nil {
			continue
		}
		fp := str(mm, "file_path")
		if fp == "" {
			continue
		}
		out = append(out, images.TMDB(fp, "w500"))
	}
	return out
}

func stillFiles(ep map[string]interface{}, n int) []string {
	img := asMap(ep["images"])
	if img == nil {
		return nil
	}
	items := asSlice(img["stills"])
	out := []string{}
	for i, it := range items {
		if i >= n {
			break
		}
		mm := asMap(it)
		if mm == nil {
			continue
		}
		fp := str(mm, "file_path")
		if fp == "" {
			continue
		}
		out = append(out, images.TMDB(fp, "w780"))
	}
	return out
}

// FirstTrailerYouTubeKey returns the first TMDB-listed YouTube trailer playback key (for embeds).
func FirstTrailerYouTubeKey(main map[string]interface{}) string {
	if main == nil {
		return ""
	}
	ts := trailers(main, 1)
	if len(ts) == 0 {
		return ""
	}
	key, _ := ts[0]["youtube_key"].(string)
	return key
}

func trailers(m map[string]interface{}, limit int) []map[string]interface{} {
	vid := asMap(m["videos"])
	if vid == nil {
		return nil
	}
	results := asSlice(vid["results"])
	out := []map[string]interface{}{}
	for _, it := range results {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		if str(mm, "type") != "Trailer" {
			continue
		}
		if str(mm, "site") != "YouTube" {
			continue
		}
		key := str(mm, "key")
		if key == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":        str(mm, "name"),
			"published":   str(mm, "published_at"),
			"youtube_key": key,
			"youtube_url": "https://www.youtube.com/watch?v=" + key,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func keywords(m map[string]interface{}) []string {
	kw := asMap(m["keywords"])
	if kw == nil {
		return nil
	}
	results := asSlice(kw["keywords"])
	out := []string{}
	for _, it := range results {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		n := str(mm, "name")
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

func ratingsMovie(m map[string]interface{}, od *omdb.Detail) map[string]interface{} {
	out := map[string]interface{}{
		"tmdb_vote_average": numFloat(m, "vote_average"),
		"tmdb_vote_count":   numInt(m, "vote_count"),
	}
	if od != nil {
		out["imdb_rating"] = od.ImdbRating
		out["imdb_votes"] = od.ImdbVotes
		out["metascore"] = od.Metascore
		out["rated"] = od.Rated
		out["imdb_id"] = od.ImdbID
	}
	return out
}

func ratingsTV(tv map[string]interface{}, od *omdb.Detail) map[string]interface{} {
	out := map[string]interface{}{
		"tmdb_vote_average": numFloat(tv, "vote_average"),
		"tmdb_vote_count":   numInt(tv, "vote_count"),
	}
	if od != nil {
		out["imdb_rating"] = od.ImdbRating
		out["imdb_votes"] = od.ImdbVotes
		out["metascore"] = od.Metascore
		out["rated"] = od.Rated
		out["imdb_id"] = od.ImdbID
	}
	return out
}

func relatedIDs(m map[string]interface{}, key string) []map[string]interface{} {
	rec := asMap(m[key])
	if rec == nil {
		return nil
	}
	results := asSlice(rec["results"])
	out := []map[string]interface{}{}
	for _, it := range results {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		title := str(mm, "title")
		name := str(mm, "name")
		display := title
		if display == "" {
			display = name
		}
		out = append(out, map[string]interface{}{
			"id":         numInt(mm, "id"),
			"title":      title,
			"name":       name,
			"display":    display,
			"poster_url": images.TMDB(str(mm, "poster_path"), "w342"),
		})
	}
	return out
}

func subtitlePayload(kind string, tmdbID int, season, episode int, tracks []subtitles.Track, langs string) map[string]interface{} {
	enriched := []map[string]interface{}{}
	for _, t := range tracks {
		q := url.Values{}
		q.Set("file_id", strconv.FormatInt(t.FileID, 10))
		q.Set("kind", kind)
		q.Set("tmdb_id", strconv.Itoa(tmdbID))
		if kind == "tv" {
			q.Set("season", strconv.Itoa(season))
			q.Set("episode", strconv.Itoa(episode))
		}
		enriched = append(enriched, map[string]interface{}{
			"file_id":   t.FileID,
			"file_name": t.FileName,
			"language":  t.Language,
			"release":   t.Release,
			"pull_url":  "/api/v1/subtitles/pull?" + q.Encode(),
		})
	}
	return map[string]interface{}{
		"filter_languages": langs,
		"tracks":           enriched,
	}
}

func episodeGuests(ep map[string]interface{}, limit int) []map[string]interface{} {
	gs := asSlice(ep["guest_stars"])
	out := []map[string]interface{}{}
	for i, it := range gs {
		if i >= limit {
			break
		}
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":          numInt(mm, "id"),
			"name":        str(mm, "name"),
			"character":   str(mm, "character"),
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
			"guest":       true,
		})
	}
	return out
}
