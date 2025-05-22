package masonry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
	"github.com/rs/xid"
	"github.com/wagoodman/go-partybus"
)

type Mason struct {
	RootPath             string
	IgnoredDirs          []string
	DaggerEnv            []string
	DaggerArgs           []string
	DaggerBinary         string
	DaggerOutputDisabled bool

	EventBus *partybus.Bus
	Logger   logger.Logger

	workspaces []Workspace
}

func NewMason() *Mason {
	return &Mason{
		Logger:      discard.New(),
		EventBus:    partybus.NewBus(),
		RootPath:    ".",
		IgnoredDirs: []string{".git"},
	}
}

func (m *Mason) DetectWorkspaces() ([]Workspace, error) {
	ignoredDirs := make(map[string]struct{})
	for _, dir := range m.IgnoredDirs {
		ignoredDirs[dir] = struct{}{}
	}

	var workspaces []Workspace

	rootPath, err := filepath.Abs(m.RootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of %q: %w", rootPath, err)
	}
	m.Logger.WithFields("dir", rootPath).Debug("Detecting workspaces")

	err = fs.WalkDir(os.DirFS(rootPath), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if _, ok := ignoredDirs[d.Name()]; ok {
			m.Logger.WithFields("dir", d.Name()).Trace("Ignoring directory")
			return fs.SkipDir
		}

		if d.IsDir() && d.Name() == MasonDirName {
			workspaceAbsPath := filepath.Join(rootPath, path, "..")
			relativePath, err := filepath.Rel(rootPath, workspaceAbsPath)
			if err != nil {
				return fmt.Errorf("failed to get relative path of %q: %w", workspaceAbsPath, err)
			}

			m.Logger.WithFields("dir", relativePath).Debug("Found workspace")
			workspaces = append(workspaces, Workspace{
				RootPath:     rootPath,
				RelativePath: relativePath,
				mason:        m,
				workDirName:  xid.New().String(),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", rootPath, err)
	}

	m.Logger.WithFields("workspaces", len(workspaces)).Info("Detected workspaces")
	m.workspaces = workspaces
	return workspaces, nil
}

func (m Mason) WorkDirs() []string {
	workDirs := make([]string, 0, len(m.workspaces))
	for _, workspace := range m.workspaces {
		workDirs = append(workDirs, workspace.WorkDir())
	}
	return workDirs
}

func (m Mason) CleanWorkDirs() error {
	var errs error
	for _, workdir := range m.WorkDirs() {
		m.Logger.WithFields("dir", workdir).Debug("Deleting work directory")
		err := os.RemoveAll(workdir)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to remove work directory %s: %w", workdir, err))
		}
	}
	if errs != nil {
		return fmt.Errorf("failed to clean work directories: %w", errs)
	}
	return nil
}
