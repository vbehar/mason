package main

import (
	"runtime/debug"

	"github.com/anchore/clio"
	"github.com/vbehar/mason/pkg/cli"
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
