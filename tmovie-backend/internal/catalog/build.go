package catalog

import (
	"fmt"

	"tmovie/internal/images"
	"tmovie/internal/omdb"
	"tmovie/internal/sources"
	"tmovie/internal/subtitles"
	"tmovie/internal/tmdb"
)

func BuildMovie(tmdbClient *tmdb.Client, omdbClient *omdb.Client, subs *subtitles.Client, id int, subtitleLangs string) (map[string]interface{}, error) {
	m, err := tmdbClient.MovieAppend(id)
	if err != nil {
		return nil, err
	}
	ext := asMap(m["external_ids"])
	imdb := str(ext, "imdb_id")

	var od *omdb.Detail
	if omdbClient != nil && imdb != "" {
		od, _ = omdbClient.ByIMDB(imdb)
	}

	var tracks []subtitles.Track
	if subs != nil && imdb != "" {
		tracks, _ = subs.SearchMovie(imdb, subtitleLangs)
	}

	return assembleMovie(id, m, od, tracks, subtitleLangs)
}

func BuildTV(tmdbClient *tmdb.Client, omdbClient *omdb.Client, subs *subtitles.Client, showID, season, episode int, subtitleLangs string) (map[string]interface{}, error) {
	tv, err := tmdbClient.TVAppend(showID)
	if err != nil {
		return nil, err
	}
	ep, err := tmdbClient.TVEpisode(showID, season, episode)
	if err != nil {
		return nil, err
	}

	tvExt := asMap(tv["external_ids"])
	epExt := asMap(ep["external_ids"])
	parentIMDB := str(tvExt, "imdb_id")
	epIMDB := str(epExt, "imdb_id")

	var od *omdb.Detail
	if omdbClient != nil && parentIMDB != "" {
		od, _ = omdbClient.ByIMDB(parentIMDB)
	}

	var tracks []subtitles.Track
	if subs != nil {
		var errTr error
		tracks, errTr = subs.SearchTVEpisode(showID, season, episode, subtitleLangs)
		if errTr != nil {
			tracks = nil
		}
		if len(tracks) == 0 && parentIMDB != "" {
			tracks, _ = subs.SearchEpisode(parentIMDB, season, episode, subtitleLangs)
		}
		if len(tracks) == 0 && epIMDB != "" {
			tracks, _ = subs.SearchMovie(epIMDB, subtitleLangs)
		}
	}

	return assembleTV(showID, season, episode, tv, ep, od, tracks, subtitleLangs)
}

func assembleMovie(tmdbID int, m map[string]interface{}, od *omdb.Detail, tracks []subtitles.Track, subtitleLangs string) (map[string]interface{}, error) {
	title := str(m, "title")
	poster := images.TMDB(str(m, "poster_path"), "w780")
	posterSmall := images.TMDB(str(m, "poster_path"), "w342")
	backdrop := images.TMDB(str(m, "backdrop_path"), "w1280")
	if backdrop == "" {
		backdrop = firstBackdropFile(m)
		if backdrop != "" {
			backdrop = images.TMDB(backdrop, "w1280")
		}
	}

	out := map[string]interface{}{
		"kind": "movie",
		"tmdb": map[string]interface{}{
			"id":              numInt(m, "id"),
			"title":           title,
			"original_title":  str(m, "original_title"),
			"overview":        str(m, "overview"),
			"tagline":         str(m, "tagline"),
			"homepage":        str(m, "homepage"),
			"status":          str(m, "status"),
			"runtime_minutes": numInt(m, "runtime"),
			"release_date":    str(m, "release_date"),
			"vote_average":    numFloat(m, "vote_average"),
			"vote_count":      numInt(m, "vote_count"),
			"popularity":      numFloat(m, "popularity"),
			"budget":          numInt(m, "budget"),
			"revenue":         numInt(m, "revenue"),
			"poster_path":     str(m, "poster_path"),
			"backdrop_path":   str(m, "backdrop_path"),
		},
		"presentation": map[string]interface{}{
			"title":                 title,
			"poster_url":            poster,
			"poster_small_url":      posterSmall,
			"backdrop_url":          backdrop,
			"overview":              str(m, "overview"),
			"release_or_air_date":   str(m, "release_date"),
			"runtime_minutes":       numInt(m, "runtime"),
			"tagline":               str(m, "tagline"),
			"homepage":              str(m, "homepage"),
			"spoken_languages":      languages(m),
			"production_countries":  countries(m),
			"production_companies":  companies(m),
		},
		"external_ids": asMap(m["external_ids"]),
		"genres":       genresNamed(m),
		"credits": map[string]interface{}{
			"cast":             castTop(m, 14),
			"crew_highlight":   crewPick(m, []string{"Director", "Creator", "Executive Producer", "Writer"}),
			"directors":        crewByJob(m, "Director"),
		},
		"images": map[string]interface{}{
			"posters":    posterFiles(m, 10),
			"backdrops":  backdropFiles(m, 8),
			"logos":      logoFiles(m, 6),
		},
		"videos": map[string]interface{}{
			"trailers": trailers(m, 6),
		},
		"keywords":     keywords(m),
		"ratings":      ratingsMovie(m, od),
		"sources":      sources.MovieEmbedURLs(tmdbID, str(asMap(m["external_ids"]), "imdb_id")),
		"subtitles":    subtitlePayload("movie", tmdbID, 0, 0, tracks, subtitleLangs),
		"recommendations": relatedIDs(m, "recommendations"),
	}

	return out, nil
}

func assembleTV(showID, season, episode int, tv, ep map[string]interface{}, od *omdb.Detail, tracks []subtitles.Track, subtitleLangs string) (map[string]interface{}, error) {
	showName := str(tv, "name")
	epTitle := str(ep, "name")
	label := fmt.Sprintf("%s S%02dE%02d", showName, season, episode)
	if epTitle != "" {
		label = label + " — " + epTitle
	}

	posterPath := str(tv, "poster_path")
	still := str(ep, "still_path")
	hero := images.TMDB(still, "w1280")
	if hero == "" {
		hero = images.TMDB(str(tv, "backdrop_path"), "w1280")
	}
	if hero == "" {
		hero = firstBackdropFile(tv)
		if hero != "" {
			hero = images.TMDB(hero, "w1280")
		}
	}

	poster := images.TMDB(posterPath, "w780")
	posterSmall := images.TMDB(posterPath, "w342")

	overview := str(ep, "overview")
	if overview == "" {
		overview = str(tv, "overview")
	}

	out := map[string]interface{}{
		"kind": "tv_episode",
		"tmdb": map[string]interface{}{
			"show_id":       showID,
			"season":        season,
			"episode":       episode,
			"show_name":     showName,
			"episode_name":  epTitle,
			"episode_type":  str(ep, "episode_type"),
			"overview":      overview,
			"air_date":      str(ep, "air_date"),
			"still_path":    still,
			"vote_average":  numFloat(tv, "vote_average"),
			"vote_count":    numInt(tv, "vote_count"),
			"episode_vote_average": numFloat(ep, "vote_average"),
			"episode_vote_count":   numInt(ep, "vote_count"),
			"runtime_minutes": avgEpisodeRuntime(tv),
			"status":         str(tv, "status"),
			"type":           str(tv, "type"),
			"homepage":       str(tv, "homepage"),
			"first_air_date": str(tv, "first_air_date"),
			"last_air_date":  str(tv, "last_air_date"),
			"number_of_seasons":    numInt(tv, "number_of_seasons"),
			"number_of_episodes":   numInt(tv, "number_of_episodes"),
			"poster_path":         str(tv, "poster_path"),
			"backdrop_path":       str(tv, "backdrop_path"),
		},
		"presentation": map[string]interface{}{
			"title":                 label,
			"show_title":            showName,
			"episode_title":         epTitle,
			"poster_url":            poster,
			"poster_small_url":      posterSmall,
			"backdrop_url":          hero,
			"still_url":             images.TMDB(still, "w780"),
			"overview":              overview,
			"release_or_air_date":   str(ep, "air_date"),
			"runtime_minutes":       numInt(ep, "runtime"),
			"homepage":              str(tv, "homepage"),
			"spoken_languages":      languages(tv),
			"production_countries":  countries(tv),
			"production_companies":  companies(tv),
			"networks":              networks(tv),
		},
		"external_ids": map[string]interface{}{
			"show":    asMap(tv["external_ids"]),
			"episode": asMap(ep["external_ids"]),
		},
		"genres": genresNamed(tv),
		"credits": map[string]interface{}{
			"cast":           castUnion(tv, ep, 14),
			"crew_highlight": crewPick(tv, []string{"Director", "Creator", "Executive Producer", "Writer"}),
			"guest_stars":    episodeGuests(ep, 12),
		},
		"images": map[string]interface{}{
			"posters":    posterFiles(tv, 10),
			"backdrops":  backdropFiles(tv, 8),
			"logos":      logoFiles(tv, 6),
			"stills":     stillFiles(ep, 8),
		},
		"videos": map[string]interface{}{
			"trailers": trailers(tv, 6),
		},
		"keywords": keywords(tv),
		"ratings":  ratingsTV(tv, od),
		"sources":  sources.TVEmbedURLs(showID, season, episode),
		"subtitles": subtitlePayload("tv", showID, season, episode, tracks, subtitleLangs),
		"recommendations": relatedIDs(tv, "recommendations"),
	}

	return out, nil
}

func avgEpisodeRuntime(tv map[string]interface{}) int {
	rt := asSlice(tv["episode_run_time"])
	if len(rt) == 0 {
		return 0
	}
	sum := 0
	for _, v := range rt {
		switch t := v.(type) {
		case float64:
			sum += int(t)
		case int:
			sum += t
		}
	}
	return sum / max(1, len(rt))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func languages(m map[string]interface{}) []map[string]string {
	out := []map[string]string{}
	for _, it := range asSlice(m["spoken_languages"]) {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]string{
			"iso_639_1": str(mm, "iso_639_1"),
			"name":      str(mm, "name"),
		})
	}
	return out
}

func countries(m map[string]interface{}) []map[string]string {
	out := []map[string]string{}
	for _, it := range asSlice(m["production_countries"]) {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]string{
			"iso_3166_1": str(mm, "iso_3166_1"),
			"name":       str(mm, "name"),
		})
	}
	return out
}

func companies(m map[string]interface{}) []map[string]string {
	out := []map[string]string{}
	for _, it := range asSlice(m["production_companies"]) {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]string{
			"name": str(mm, "name"),
			"logo": images.TMDB(str(mm, "logo_path"), "w185"),
		})
	}
	return out
}

func networks(tv map[string]interface{}) []map[string]string {
	out := []map[string]string{}
	for _, it := range asSlice(tv["networks"]) {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]string{
			"name": str(mm, "name"),
			"logo": images.TMDB(str(mm, "logo_path"), "w185"),
		})
	}
	return out
}

func genresNamed(m map[string]interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, it := range asSlice(m["genres"]) {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":   numInt(mm, "id"),
			"name": str(mm, "name"),
		})
	}
	return out
}

func castTop(m map[string]interface{}, limit int) []map[string]interface{} {
	cr := asMap(m["credits"])
	if cr == nil {
		return nil
	}
	cast := asSlice(cr["cast"])
	out := []map[string]interface{}{}
	for i, it := range cast {
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
			"order":       numInt(mm, "order"),
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
		})
	}
	return out
}

func castUnion(tv, ep map[string]interface{}, limit int) []map[string]interface{} {
	out := []map[string]interface{}{}
	seen := map[int]struct{}{}
	ch := asMap(tv["credits"])
	var cast []interface{}
	if ch != nil {
		cast = asSlice(ch["cast"])
	}
	for _, it := range cast {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		id := numInt(mm, "id")
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, map[string]interface{}{
			"id":          id,
			"name":        str(mm, "name"),
			"character":   str(mm, "character"),
			"order":       numInt(mm, "order"),
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
		})
		if len(out) >= limit {
			return out
		}
	}
	gs := asSlice(ep["guest_stars"])
	for _, it := range gs {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		id := numInt(mm, "id")
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, map[string]interface{}{
			"id":          id,
			"name":        str(mm, "name"),
			"character":   str(mm, "character"),
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
			"guest":       true,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func crewPick(m map[string]interface{}, jobs []string) []map[string]interface{} {
	cr := asMap(m["credits"])
	if cr == nil {
		return nil
	}
	want := map[string]struct{}{}
	for _, j := range jobs {
		want[j] = struct{}{}
	}
	crew := asSlice(cr["crew"])
	out := []map[string]interface{}{}
	for _, it := range crew {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		job := str(mm, "job")
		if _, ok := want[job]; !ok {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":          numInt(mm, "id"),
			"name":        str(mm, "name"),
			"job":         job,
			"department":  str(mm, "department"),
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
		})
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func crewByJob(m map[string]interface{}, job string) []map[string]interface{} {
	cr := asMap(m["credits"])
	if cr == nil {
		return nil
	}
	crew := asSlice(cr["crew"])
	out := []map[string]interface{}{}
	for _, it := range crew {
		mm := asMap(it)
		if mm == nil {
			continue
		}
		if str(mm, "job") != job {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":          numInt(mm, "id"),
			"name":        str(mm, "name"),
			"job":         job,
			"profile_url": images.TMDB(str(mm, "profile_path"), "h632"),
		})
	}
	return out
}
