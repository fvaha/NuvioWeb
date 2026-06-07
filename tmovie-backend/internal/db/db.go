package db

import (
	"os"
	"path/filepath"

	"tmovie/internal/models"

	// Pure-Go sqlite driver (modernc). Avoids a CGO sqlite symbol clash with
	// the CGO sqlite pulled in transitively by the torrent engine.
	gormsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dialector := gormsqlite.Open(path)
	gdb, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := gdb.AutoMigrate(&models.CachedMovie{}); err != nil {
		return nil, err
	}
	return gdb, nil
}
