package main

import (
	"runtime/debug"

	"github.com/anchore/clio"
	"github.com/vbehar/mason/pkg/cli"
	"golang.org/x/mod/module"
)

const applicationName = "mason"

// all variables here are provided as build-time arguments
var (
	version        string
	buildDate      string
	gitCommit      string
	gitDescription string
)

func main() {
	cli.Application(appID()).Run()
}

func appID() clio.Identification {
	if version == "" {
		version = defaultVersion()
	}
	if buildDate == "" {
		buildDate = defaultBuildDate()
	}
	if gitCommit == "" {
		gitCommit = defaultGitCommit()
	}
	return clio.Identification{
		Name:           applicationName,
		Version:        version,
		GitCommit:      gitCommit,
		GitDescription: gitDescription,
		BuildDate:      buildDate,
	}
}

func defaultVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" {
			return v
		}
	}
	return ""
}

func defaultBuildDate() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && module.IsPseudoVersion(v) {
			t, err := module.PseudoVersionTime(v)
			if err == nil {
				return t.Format("2006-01-02T15:04:05Z07:00")
			}
		}
	}
	return ""
}

func defaultGitCommit() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && module.IsPseudoVersion(v) {
			rev, err := module.PseudoVersionRev(v)
			if err == nil {
				return rev
			}
		}
	}
	return ""
}
