package masonry

import (
	"testing"
)

func TestModuleRefSanitizedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    ModuleRef
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "gitlab ssh url",
			input:    "git@gitlab.com:user/repo.git@v1.2.3",
			expected: "gitlab_com_user_repo_v1_2_3",
		},
		{
			name:     "github ssh url",
			input:    "git@github.com:user/repo.git",
			expected: "github_com_user_repo",
		},
		{
			name:     "github url",
			input:    "https://github.com/user/repo@v1.2.3",
			expected: "github_com_user_repo_v1_2_3",
		},
		{
			name:     "local relative path",
			input:    ".mason/modules/module",
			expected: "_mason_modules_module",
		},
		{
			name:     "local absolute path",
			input:    "/path/to/mason/module",
			expected: "_path_to_mason_module",
		},
		{
			name:     "special characters",
			input:    "module-name.with.special_chars",
			expected: "module_name_with_special_chars",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.input.SanitizedName()
			if actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
