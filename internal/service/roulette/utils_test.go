package roulette

import (
	"testing"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		file     string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "README.md",
			file:     "README.md",
			expected: true,
		},
		{
			name:     "wildcard extension",
			pattern:  "*.go",
			file:     "main.go",
			expected: true,
		},
		{
			name:     "wildcard extension no match",
			pattern:  "*.go",
			file:     "main.js",
			expected: false,
		},
		{
			name:     "wildcard all",
			pattern:  "*",
			file:     "anything.txt",
			expected: true,
		},
		{
			name:     "complex pattern",
			pattern:  "*.test.js",
			file:     "component.test.js",
			expected: true,
		},
		{
			name:     "complex pattern no match",
			pattern:  "*.test.js",
			file:     "component.js",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.file)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.file, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			slice:    []string{"alice", "bob", "charlie"},
			item:     "bob",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"alice", "bob", "charlie"},
			item:     "david",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "alice",
			expected: false,
		},
		{
			name:     "single item match",
			slice:    []string{"alice"},
			item:     "alice",
			expected: true,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Alice"},
			item:     "alice",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, result, tt.expected)
			}
		})
	}
}

func TestSelectByScore(t *testing.T) {
	tests := []struct {
		name      string
		reviewers []*Reviewer
		expectNil bool
		expectID  uint
	}{
		{
			name:      "empty list",
			reviewers: []*Reviewer{},
			expectNil: true,
		},
		{
			name: "single reviewer",
			reviewers: []*Reviewer{
				{User: &models.User{ID: 1, Username: "alice"}, Score: 100.0},
			},
			expectNil: false,
			expectID:  1,
		},
		{
			name: "select highest score",
			reviewers: []*Reviewer{
				{User: &models.User{ID: 1, Username: "alice"}, Score: 80.0},
				{User: &models.User{ID: 2, Username: "bob"}, Score: 95.0},
				{User: &models.User{ID: 3, Username: "charlie"}, Score: 70.0},
			},
			expectNil: false,
			expectID:  2,
		},
		{
			name: "equal scores - returns one of them",
			reviewers: []*Reviewer{
				{User: &models.User{ID: 1, Username: "alice"}, Score: 90.0},
				{User: &models.User{ID: 2, Username: "bob"}, Score: 90.0},
			},
			expectNil: false,
			// Should return one of them (random), we just check it's not nil
		},
		{
			name: "negative scores",
			reviewers: []*Reviewer{
				{User: &models.User{ID: 1, Username: "alice"}, Score: -10.0},
				{User: &models.User{ID: 2, Username: "bob"}, Score: 0.0},
			},
			expectNil: false,
			expectID:  2, // bob has higher score (0 > -10)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectByScore(tt.reviewers)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected non-nil result")
				return
			}

			// For equal scores test, just verify we got a result
			if tt.name == "equal scores - returns one of them" {
				found := false
				for _, r := range tt.reviewers {
					if result.User.ID == r.User.ID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("result not in original list")
				}
				return
			}

			if result.User.ID != tt.expectID {
				t.Errorf("expected user ID %d, got %d", tt.expectID, result.User.ID)
			}
		})
	}
}
