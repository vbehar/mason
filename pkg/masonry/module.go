package masonry

import (
	"net/url"
	"path/filepath"
	"strings"
)

type ModuleRef string

func (m ModuleRef) SanitizedName() string {
	name := string(m)

	if u, _ := url.Parse(name); u != nil {
		name = filepath.Join(u.Hostname(), u.Path)
	}

	name = strings.TrimSuffix(name, ".git")

	name = strings.ReplaceAll(name, "https://", "")
	name = strings.ReplaceAll(name, "http://", "")
	name = strings.ReplaceAll(name, "git@", "")

	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "@", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")

	return name
}
