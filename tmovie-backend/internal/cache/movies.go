package cache

import (
	"time"

	"tmovie/internal/models"

	"gorm.io/gorm"
)

func UpsertMovie(db *gorm.DB, m models.CachedMovie) error {
	m.LastFetched = time.Now()
	var existing models.CachedMovie
	tx := db.Where("tmdb_id = ?", m.TMDBID).First(&existing)
	if tx.Error == nil {
		m.ID = existing.ID
		return db.Save(&m).Error
	}
	return db.Create(&m).Error
}
