package images

import "strings"

const Base = "https://image.tmdb.org/t/p/"

func TMDB(path, profile string) string {
	if path == "" {
		return ""
	}
	p := strings.TrimSpace(path)
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	return Base + profile + p
}
