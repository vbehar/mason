package masonry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vbehar/mason/pkg/dagger"
)

type Script struct {
	ModuleName string
	Phase      string
	PostRun    PostRun
	Name       string
	Content    dagger.Script
}

func ScriptFromFile(filePath string) (*Script, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	fileName := filepath.Base(filePath)
	fileName = strings.TrimSuffix(fileName, ".dagger")
	dir := filepath.Dir(filePath)
	dirName := filepath.Base(dir)

	phase, name, ok := strings.Cut(fileName, "_")
	if !ok {
		phase = "" // meaning all phases
		name = fileName
	}
	if phase == "postrun" {
		phase = ""
		name = fileName
	}

	var postRun PostRun
	switch {
	case strings.HasPrefix(name, "postrun_on_success_"):
		name = strings.TrimPrefix(name, "postrun_on_success_")
		postRun = PostRunOnSuccess
	case strings.HasPrefix(name, "postrun_on_failure_"):
		name = strings.TrimPrefix(name, "postrun_on_failure_")
		postRun = PostRunOnFailure
	case strings.HasPrefix(name, "postrun_"):
		name = strings.TrimPrefix(name, "postrun_")
		postRun = PostRunAlways
	}

	return &Script{
		ModuleName: dirName,
		Phase:      phase,
		PostRun:    postRun,
		Name:       name,
		Content:    dagger.Script(content),
	}, nil
}

func (s Script) Equals(other Script) bool {
	return s.ModuleName == other.ModuleName &&
		s.Phase == other.Phase &&
		s.PostRun == other.PostRun &&
		s.Name == other.Name &&
		s.Content == other.Content
}
