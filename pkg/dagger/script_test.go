package dagger

import (
	"maps"
	"testing"
)

func TestScriptExtractDefinedVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		expected map[string]struct{}
	}{
		{
			name:     "empty script",
			script:   "",
			expected: map[string]struct{}{},
		},
		{
			name:   "single variable",
			script: "foo=$(do something)",
			expected: map[string]struct{}{
				"foo": {},
			},
		},
		{
			name: "multiple variables",
			script: `foo=$(do something)
bar=$(do something else)`,
			expected: map[string]struct{}{
				"foo": {},
				"bar": {},
			},
		},
		{
			name:   "with whitespace",
			script: `  foo  = $(do something) `,
			expected: map[string]struct{}{
				"foo": {},
			},
		},
		{
			name: "mixed content",
			script: `#!/usr/bin/env dagger
container | from alpine | file /etc/alpine-release | contents
alpine_ctr=$(container | from alpine)
container | from debian | file /etc/debian_version | contents
debian_ctr=$(container | from debian)
$alpine_ctr | file "/etc/alpine-release" | contents
$debian_ctr | file "/etc/debian_version" | contents`,
			expected: map[string]struct{}{
				"alpine_ctr": {},
				"debian_ctr": {},
			},
		},
		{
			name: "complex script",
			script: `mason_linux_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os linux --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_linux_arm64)
$mason_linux_arm64 | export bin/mason-linux-arm64`,
			expected: map[string]struct{}{
				"mason_linux_arm64": {},
			},
		},
		{
			name:     "used variables only",
			script:   `$alpine_ctr | file "/etc/alpine-release" | contents`,
			expected: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := Script(tt.script).ExtractDefinedVariables()
			if !maps.Equal(actual, tt.expected) {
				t.Errorf("ExtractDefinedVariables() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestScriptExtractUsedVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		expected map[string]struct{}
	}{
		{
			name:     "empty script",
			script:   "",
			expected: map[string]struct{}{},
		},
		{
			name:   "single variable",
			script: "container | from $foo",
			expected: map[string]struct{}{
				"foo": {},
			},
		},
		{
			name: "multiple variables",
			script: `container | from $foo
container | from $bar`,
			expected: map[string]struct{}{
				"foo": {},
				"bar": {},
			},
		},
		{
			name:   "with whitespace",
			script: `  container | from  $foo  `,
			expected: map[string]struct{}{
				"foo": {},
			},
		},
		{
			name: "mixed content",
			script: `#!/usr/bin/env dagger
container | from alpine | file /etc/alpine-release | contents
alpine_ctr=$(container | from alpine)
container | from debian | file /etc/debian_version | contents
debian_ctr=$(container | from debian)
$alpine_ctr | file "/etc/alpine-release" | contents
$debian_ctr | file "/etc/debian_version" | contents`,
			expected: map[string]struct{}{
				"alpine_ctr": {},
				"debian_ctr": {},
			},
		},
		{
			name: "complex script",
			script: `mason_linux_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os linux --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_linux_arm64)
$mason_linux_arm64 | export bin/mason-linux-arm64`,
			expected: map[string]struct{}{
				"mason_linux_arm64": {},
			},
		},
		{
			name:     "defined variables only",
			script:   `alpine_ctr=$(container | from alpine)`,
			expected: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := Script(tt.script).ExtractUsedVariables()
			if !maps.Equal(actual, tt.expected) {
				t.Errorf("ExtractUsedVariables() = %v, want %v", actual, tt.expected)
			}
		})
	}
}
