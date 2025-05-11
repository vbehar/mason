package dagger

import (
	"regexp"
)

var (
	varDefinitionRegex = regexp.MustCompile(`\s*([a-zA-Z][a-zA-Z0-9_]*)\s*=\s*\$\(`)
	varUsageRegex      = regexp.MustCompile(`\$([a-zA-Z][a-zA-Z0-9_]*)`)
)

type Script string

func (s Script) ExtractDefinedVariables() map[string]struct{} {
	return s.extractVariables(varDefinitionRegex)
}

func (s Script) ExtractUsedVariables() map[string]struct{} {
	return s.extractVariables(varUsageRegex)
}

func (s Script) extractVariables(r *regexp.Regexp) map[string]struct{} {
	variables := make(map[string]struct{})
	for _, matches := range r.FindAllStringSubmatch(string(s), -1) {
		if len(matches) != 2 {
			continue
		}

		varName := matches[1]
		variables[varName] = struct{}{}
	}
	return variables
}
