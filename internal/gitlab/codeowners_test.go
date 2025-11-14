package gitlab

import (
	"testing"
)

func TestParseCodeowners(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string][]string
	}{
		{
			name:    "simple pattern with single owner",
			content: `* @alice`,
			expected: map[string][]string{
				"*": {"alice"},
			},
		},
		{
			name:    "multiple owners for pattern",
			content: `*.js @bob @charlie`,
			expected: map[string][]string{
				"*.js": {"bob", "charlie"},
			},
		},
		{
			name: "multiple patterns",
			content: `*.go @david
*.py @eve @frank`,
			expected: map[string][]string{
				"*.go": {"david"},
				"*.py": {"eve", "frank"},
			},
		},
		{
			name: "with comments and empty lines",
			content: `# This is a comment

* @alice
# Another comment
*.js @bob`,
			expected: map[string][]string{
				"*":    {"alice"},
				"*.js": {"bob"},
			},
		},
		{
			name: "path patterns",
			content: `/docs/ @alice
/api/ @bob @charlie`,
			expected: map[string][]string{
				"/docs/": {"alice"},
				"/api/":  {"bob", "charlie"},
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: map[string][]string{},
		},
		{
			name:     "only comments",
			content:  "# Comment 1\n# Comment 2",
			expected: map[string][]string{},
		},
		{
			name:     "invalid lines without owners",
			content:  "*.js\n*.go",
			expected: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCodeowners(tt.content)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d patterns, got %d", len(tt.expected), len(result))
				return
			}

			// Check each pattern
			for pattern, expectedOwners := range tt.expected {
				actualOwners, exists := result[pattern]
				if !exists {
					t.Errorf("pattern %q not found in result", pattern)
					continue
				}

				if len(actualOwners) != len(expectedOwners) {
					t.Errorf("pattern %q: expected %d owners, got %d", pattern, len(expectedOwners), len(actualOwners))
					continue
				}

				// Check owners match
				for i, expectedOwner := range expectedOwners {
					if actualOwners[i] != expectedOwner {
						t.Errorf("pattern %q: expected owner[%d] = %q, got %q", pattern, i, expectedOwner, actualOwners[i])
					}
				}
			}
		})
	}
}
