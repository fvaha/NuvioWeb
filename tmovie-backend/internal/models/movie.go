package models

import "time"

type CachedMovie struct {
	ID          uint `gorm:"primaryKey"`
	TMDBID      int  `gorm:"uniqueIndex;not null"`
	Title       string
	PosterPath  string
	Overview    string
	ReleaseDate string
	MediaType   string `gorm:"default:movie"`
	LastFetched time.Time
}
