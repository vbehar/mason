package masonry

import (
	"strings"
	"testing"
)

func TestPlanComputeMergedScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		sourceScripts []Script
		expected      string
		expectedError string
	}{
		{
			name:          "empty source scripts",
			sourceScripts: []Script{},
			expected:      "",
		},
		{
			name: "single source script",
			sourceScripts: []Script{
				{
					Name: "SingleSourceScript",
					Content: `
my_container=$(container | from alpine)
$my_container | file "/etc/alpine-release" | contents
`,
				},
			},
			expected: `#!/usr/bin/env dagger

# SingleSourceScript
my_container=$(container | from alpine)
$my_container | file "/etc/alpine-release" | contents
.echo`,
		},
		{
			name: "multiple scripts without variables",
			sourceScripts: []Script{
				{
					Name:    "AlpineScript",
					Content: "container | from alpine | file /etc/alpine-release | contents",
				},
				{
					Name:    "DebianScript",
					Content: "container | from debian | file /etc/debian_version | contents",
				},
			},
			expected: `#!/usr/bin/env dagger

# AlpineScript
container | from alpine | file /etc/alpine-release | contents
.echo

# DebianScript
container | from debian | file /etc/debian_version | contents
.echo`,
		},
		{
			name: "multiple scripts with variables",
			sourceScripts: []Script{
				{
					Name: "AlpineScript",
					Content: `
alpine_ctr=$(container | from alpine)
$alpine_ctr | file "/etc/alpine-release" | contents
`,
				},
				{
					Name: "DebianScript",
					Content: `
debian_ctr=$(container | from debian)
$debian_ctr | file "/etc/debian_version" | contents
`,
				},
			},
			expected: `#!/usr/bin/env dagger

# AlpineScript
alpine_ctr=$(container | from alpine)
$alpine_ctr | file "/etc/alpine-release" | contents
.echo

# DebianScript
debian_ctr=$(container | from debian)
$debian_ctr | file "/etc/debian_version" | contents
.echo`,
		},
		{
			name: "multiple scripts with re-used variables",
			sourceScripts: []Script{
				{
					Name:    "Script1",
					Content: `debian_ctr=$(container | from debian); alpine_os_release_file=$($alpine_ctr | file "/etc/os-release")`,
				},
				{
					Name:    "Script2",
					Content: "alpine_etc_dir=$($alpine_ctr | directory /etc)",
				},
				{
					Name:    "Script3",
					Content: `$alpine_etc_dir | file "alpine-release" | export "/path/to/alpine_release"`,
				},
				{
					Name:    "Script4",
					Content: "alpine_ctr=$(container | from alpine)",
				},
				{
					Name: "Script5",
					Content: `
$debian_ctr | file "/etc/debian_version" | export "/path/to/debian_version"
$alpine_os_release_file | export "/path/to/alpine_release"
`,
				},
				{
					Name:    "Script6",
					Content: `$alpine_ctr | file "/etc/alpine-release" | export "/path/to/alpine_release"`,
				},
			},
			expected: `#!/usr/bin/env dagger

# Script4
alpine_ctr=$(container | from alpine)
.echo

# Script6
$alpine_ctr | file "/etc/alpine-release" | export "/path/to/alpine_release"
.echo

# Script2
alpine_etc_dir=$($alpine_ctr | directory /etc)
.echo

# Script3
$alpine_etc_dir | file "alpine-release" | export "/path/to/alpine_release"
.echo

# Script1
debian_ctr=$(container | from debian); alpine_os_release_file=$($alpine_ctr | file "/etc/os-release")
.echo

# Script5
$debian_ctr | file "/etc/debian_version" | export "/path/to/debian_version"
$alpine_os_release_file | export "/path/to/alpine_release"
.echo`,
		},
		{
			name: "complex script",
			sourceScripts: []Script{
				{
					Name: "linux_arm64",
					Content: `mason_linux_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os linux --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_linux_arm64)
$mason_linux_arm64 | export bin/mason-linux-arm64`,
				},
				{
					Name: "darwin_arm64",
					Content: `mason_darwin_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os darwin --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_darwin_arm64)
$mason_darwin_arm64 | export bin/mason-darwin-arm64`,
				},
			},
			expected: `#!/usr/bin/env dagger

# darwin_arm64
mason_darwin_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os darwin --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_darwin_arm64)
$mason_darwin_arm64 | export bin/mason-darwin-arm64
.echo

# linux_arm64
mason_linux_arm64=$(https://github.com/vbehar/mason-modules/golang $(host | directory . --exclude ".history",".mason","bin") | build-binary --go-os linux --go-arch arm64 --args "-ldflags","-X main.version=1.0.0" --output-file-name mason_linux_arm64)
$mason_linux_arm64 | export bin/mason-linux-arm64
.echo`,
		},
		{
			name: "variable defined twice",
			sourceScripts: []Script{
				{
					Name:    "Script1",
					Content: "alpine_ctr=$(container | from alpine)",
				},
				{
					Name:    "Script2",
					Content: "alpine_ctr=$(container | from ubuntu)",
				},
			},
			expectedError: `variable "alpine_ctr" is defined twice: by "Script1" and "Script2"`,
		},
		{
			name: "variable used but not defined",
			sourceScripts: []Script{
				{
					Name:    "Script",
					Content: "$missing_var | do something",
				},
			},
			expectedError: `variable "missing_var" is used but not defined. Used by [Script]`,
		},
		{
			name: "circular dependency",
			sourceScripts: []Script{
				{
					Name:    "Script1",
					Content: "a=$(echo $b)",
				},
				{
					Name:    "Script2",
					Content: "b=$(echo $a)",
				},
			},
			expectedError: `would create a loop`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := &Plan{
				SourceScripts: tt.sourceScripts,
			}
			err := plan.computeMergedScript()
			if err != nil {
				if tt.expectedError == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Fatalf("expected error: %v, got: %v", tt.expectedError, err)
				}
			} else {
				if tt.expectedError != "" {
					t.Fatalf("expected error: %v, got a merged script:\n%s", tt.expectedError, plan.MergedScript)
				} else if plan.MergedScript != tt.expected {
					t.Errorf("expected merged script:\n%s\n\ngot:\n%s", tt.expected, plan.MergedScript)
				}
			}
		})
	}
}
