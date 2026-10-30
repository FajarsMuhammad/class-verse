// Package maps embeds the Tiled map files (.tmj) so the server binary carries
// the same maps it serves to the frontend (GET /api/maps/{map_key}).
package maps

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
)

//go:embed *.tmj
var files embed.FS

var keyRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ErrUnknown is returned for a map key without a file.
var ErrUnknown = errors.New("unknown map")

// Read returns the raw .tmj bytes for key.
func Read(key string) ([]byte, error) {
	if !keyRe.MatchString(key) {
		return nil, ErrUnknown
	}
	b, err := files.ReadFile(key + ".tmj")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, fmt.Errorf("read map %s: %w", key, err)
	}
	return b, nil
}
