package masonry

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
	"github.com/rs/xid"
	"github.com/wagoodman/go-partybus"
)

type Mason struct {
	RootPath    string
	IgnoredDirs []string
	DaggerOut   io.Writer // this is the stderr of the dagger process
	DaggerEnv   []string
	DaggerArgs  []string

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
		DaggerOut:   os.Stderr, // dagger's default is stderr, let's keep it that way
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

func (m Mason) CleanWorkDirs() error {
	var errs error
	for _, workspace := range m.workspaces {
		m.Logger.WithFields("dir", workspace.WorkDir()).Debug("Deleting work directory")
		err := os.RemoveAll(workspace.WorkDir())
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to remove work directory %s: %w", workspace.WorkDir(), err))
		}
	}
	if errs != nil {
		return fmt.Errorf("failed to clean work directories: %w", errs)
	}
	return nil
}
