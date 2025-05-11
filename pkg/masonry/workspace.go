package masonry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anchore/go-logger"
	"github.com/goccy/go-yaml"
)

type Workspace struct {
	RootPath     string
	RelativePath string // relative to the workspace root

	mason       *Mason
	workDirName string
}

func (w Workspace) logger() logger.Logger {
	return w.mason.Logger.Nested("workspace", w.RelativePath)
}

func (w Workspace) Dir() string {
	return filepath.Join(w.RootPath, w.RelativePath)
}

func (w Workspace) MasonDir() string {
	return filepath.Join(w.Dir(), MasonDirName)
}

func (w Workspace) WorkDir() string {
	return filepath.Join(w.MasonDir(), WorkDirPrefix, w.workDirName)
}

func (w Workspace) LoadBlueprint() (*Blueprint, error) {
	w.logger().Debug("Loading blueprint")
	entries, err := os.ReadDir(w.MasonDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", w.MasonDir(), err)
	}

	var bricks []Brick
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if entry.Name() == "config.yaml" {
			continue // mason's own config file...
		}

		switch filepath.Ext(entry.Name()) {
		case ".json", ".yaml", ".yml":
			// valid file types
		default:
			continue
		}

		w.logger().WithFields("file", entry.Name()).Trace("Loading file")
		f, err := os.Open(filepath.Join(w.MasonDir(), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", entry.Name(), err)
		}
		defer f.Close() //nolint:errcheck // we're just reading the file...

		decoder := yaml.NewDecoder(f)
		for doc := 0; ; doc++ {
			var brick Brick
			err = decoder.Decode(&brick)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to decode document %d from file %s: %w", doc, entry.Name(), err)
			}
			if brick.IsValid() {
				w.logger().WithFields("name", brick.Metadata.Name, "kind", brick.Kind).
					Debug("Loaded brick")
				bricks = append(bricks, brick)
			}
		}
	}

	w.logger().WithFields("bricks", len(bricks)).Info("Loaded bricks")
	return &Blueprint{
		Bricks:    bricks,
		workspace: w,
	}, nil
}
