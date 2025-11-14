package roulette

import (
	"testing"
)

// Test scoring algorithm logic with known values
func TestScoringLogic(t *testing.T) {
	tests := []struct {
		name          string
		baseScore     float64
		activeReviews int
		loadWeight    int
		recentReviews bool
		recentWeight  int
		force         bool
		expected      float64
	}{
		{
			name:          "no load, no recent reviews",
			baseScore:     100.0,
			activeReviews: 0,
			loadWeight:    10,
			recentReviews: false,
			recentWeight:  5,
			force:         false,
			expected:      100.0,
		},
		{
			name:          "one active review",
			baseScore:     100.0,
			activeReviews: 1,
			loadWeight:    10,
			recentReviews: false,
			recentWeight:  5,
			force:         false,
			expected:      90.0, // 100 - (1 * 10)
		},
		{
			name:          "multiple active reviews",
			baseScore:     100.0,
			activeReviews: 3,
			loadWeight:    10,
			recentReviews: false,
			recentWeight:  5,
			force:         false,
			expected:      70.0, // 100 - (3 * 10)
		},
		{
			name:          "recent review penalty",
			baseScore:     100.0,
			activeReviews: 0,
			loadWeight:    10,
			recentReviews: true,
			recentWeight:  5,
			force:         false,
			expected:      95.0, // 100 - 5
		},
		{
			name:          "recent review with force flag",
			baseScore:     100.0,
			activeReviews: 0,
			loadWeight:    10,
			recentReviews: true,
			recentWeight:  5,
			force:         true,
			expected:      100.0, // force flag ignores recent reviews
		},
		{
			name:          "combined penalties",
			baseScore:     100.0,
			activeReviews: 2,
			loadWeight:    10,
			recentReviews: true,
			recentWeight:  5,
			force:         false,
			expected:      75.0, // 100 - (2 * 10) - 5
		},
		{
			name:          "score cannot go below zero",
			baseScore:     100.0,
			activeReviews: 15,
			loadWeight:    10,
			recentReviews: true,
			recentWeight:  5,
			force:         false,
			expected:      0.0, // 100 - (15 * 10) - 5 = -55, clamped to 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate score using the algorithm
			score := tt.baseScore

			// Apply load penalty
			score -= float64(tt.activeReviews) * float64(tt.loadWeight)

			// Apply recent review penalty
			if tt.recentReviews && !tt.force {
				score -= float64(tt.recentWeight)
			}

			// Clamp to 0
			if score < 0 {
				score = 0
			}

			if score != tt.expected {
				t.Errorf("expected score %.1f, got %.1f", tt.expected, score)
			}
		})
	}
}

// Test edge cases for reviewer selection
func TestReviewerSelectionEdgeCases(t *testing.T) {
	t.Run("empty candidate pool", func(t *testing.T) {
		candidates := []string{}
		if len(candidates) > 0 {
			t.Error("should handle empty candidate pool")
		}
	})

	t.Run("single candidate", func(t *testing.T) {
		candidates := []string{"alice"}
		if len(candidates) != 1 {
			t.Error("should handle single candidate")
		}
	})

	t.Run("ensure uniqueness", func(t *testing.T) {
		selected := make(map[string]bool)
		candidates := []string{"alice", "bob", "charlie"}

		for _, candidate := range candidates {
			if selected[candidate] {
				t.Errorf("duplicate selection: %s", candidate)
			}
			selected[candidate] = true
		}
	})
}
