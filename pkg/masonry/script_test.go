package masonry

import (
	"strings"
	"testing"
)

func TestScriptFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		filePath      string
		expected      Script
		expectedError string
	}{
		{
			name:     "single phase script",
			filePath: "testdata/golang/build_binary.dagger",
			expected: Script{
				Name:       "binary",
				ModuleName: "golang",
				Phase:      "build",
				Content:    `.echo "build some binary"`,
			},
		},
		{
			name:     "all phases script",
			filePath: "testdata/golang/generic.dagger",
			expected: Script{
				Name:       "generic",
				ModuleName: "golang",
				Content:    `.echo "generic script"`,
			},
		},
		{
			name:     "postrun on success script",
			filePath: "testdata/golang/postrun_on_success_one.dagger",
			expected: Script{
				Name:       "one",
				ModuleName: "golang",
				PostRun:    PostRunOnSuccess,
				Content:    `.echo "postrun on success script"`,
			},
		},
		{
			name:     "build postrun on failure script",
			filePath: "testdata/golang/build_postrun_on_failure_two.dagger",
			expected: Script{
				Name:       "two",
				ModuleName: "golang",
				Phase:      "build",
				PostRun:    PostRunOnFailure,
				Content:    `.echo "build postrun on failure script"`,
			},
		},
		{
			name:     "postrun script",
			filePath: "testdata/golang/postrun_something.dagger",
			expected: Script{
				Name:       "something",
				ModuleName: "golang",
				PostRun:    PostRunAlways,
				Content:    `.echo "postrun script"`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			script, err := ScriptFromFile(test.filePath)
			if err != nil {
				if test.expectedError == "" {
					t.Fatalf("unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), test.expectedError) {
					t.Fatalf("expected error to contain %q, got %v", test.expectedError, err)
				}
				return
			}

			if script.Name != test.expected.Name {
				t.Errorf("expected name %q, got %q", test.expected.Name, script.Name)
			}
			if script.Phase != test.expected.Phase {
				t.Errorf("expected phase %q, got %q", test.expected.Phase, script.Phase)
			}
			if script.ModuleName != test.expected.ModuleName {
				t.Errorf("expected module name %q, got %q", test.expected.ModuleName, script.ModuleName)
			}
			if script.PostRun != test.expected.PostRun {
				t.Errorf("expected post run %q, got %q", test.expected.PostRun, script.PostRun)
			}
			if script.Content != test.expected.Content {
				t.Errorf("expected content %q, got %q", test.expected.Content, script.Content)
			}
		})
	}
}
