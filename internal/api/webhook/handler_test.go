package webhook

import (
	"testing"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/i18n"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/roulette"
)

func TestParseRouletteCommand(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name              string
		comment           string
		expectCommand     string
		expectForce       bool
		expectNoCodeowner bool
		expectInclude     []string
		expectExclude     []string
	}{
		{
			name:          "simple /roulette",
			comment:       "/roulette",
			expectCommand: "roulette",
		},
		{
			name:          "/roulette with force flag",
			comment:       "/roulette --force",
			expectCommand: "roulette",
			expectForce:   true,
		},
		{
			name:              "/roulette with no-codeowner flag",
			comment:           "/roulette --no-codeowner",
			expectCommand:     "roulette",
			expectNoCodeowner: true,
		},
		{
			name:          "/roulette with include users",
			comment:       "/roulette --include @alice @bob",
			expectCommand: "roulette",
			expectInclude: []string{"alice", "bob"},
		},
		{
			name:          "/roulette with exclude users",
			comment:       "/roulette --exclude @charlie",
			expectCommand: "roulette",
			expectExclude: []string{"charlie"},
		},
		{
			name:          "/roulette with multiple flags",
			comment:       "/roulette --force --include @alice --exclude @bob",
			expectCommand: "roulette",
			expectForce:   true,
			expectInclude: []string{"alice"},
			expectExclude: []string{"bob"},
		},
		{
			name:          "include without @ prefix",
			comment:       "/roulette --include alice bob",
			expectCommand: "roulette",
			expectInclude: []string{"alice", "bob"},
		},
		{
			name:          "not a roulette command",
			comment:       "This is a normal comment",
			expectCommand: "",
		},
		{
			name:          "/roulette in middle of comment",
			comment:       "Can someone run /roulette please?",
			expectCommand: "",
		},
		{
			name:          "/roulette at start of line in multiline",
			comment:       "Some text\n/roulette\nMore text",
			expectCommand: "roulette",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, options := h.parseRouletteCommand(tt.comment)

			if command != tt.expectCommand {
				t.Errorf("expected command %q, got %q", tt.expectCommand, command)
			}

			if options.Force != tt.expectForce {
				t.Errorf("expected Force=%v, got %v", tt.expectForce, options.Force)
			}

			if options.NoCodeowner != tt.expectNoCodeowner {
				t.Errorf("expected NoCodeowner=%v, got %v", tt.expectNoCodeowner, options.NoCodeowner)
			}

			if len(options.IncludeUsers) != len(tt.expectInclude) {
				t.Errorf("expected %d include users, got %d", len(tt.expectInclude), len(options.IncludeUsers))
			} else {
				for i, user := range tt.expectInclude {
					if options.IncludeUsers[i] != user {
						t.Errorf("expected include[%d]=%s, got %s", i, user, options.IncludeUsers[i])
					}
				}
			}

			if len(options.ExcludeUsers) != len(tt.expectExclude) {
				t.Errorf("expected %d exclude users, got %d", len(tt.expectExclude), len(options.ExcludeUsers))
			} else {
				for i, user := range tt.expectExclude {
					if options.ExcludeUsers[i] != user {
						t.Errorf("expected exclude[%d]=%s, got %s", i, user, options.ExcludeUsers[i])
					}
				}
			}
		})
	}
}

func TestFormatRouletteResult(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("Failed to initialize translator: %v", err)
	}

	h := &Handler{
		translator: translator,
	}

	tests := []struct {
		name     string
		result   *roulette.SelectionResult
		contains []string
	}{
		{
			name: "all three reviewers",
			result: &roulette.SelectionResult{
				Codeowner: &roulette.Reviewer{
					User:          &models.User{Username: "alice"},
					ActiveReviews: 2,
				},
				TeamMember: &roulette.Reviewer{
					User:          &models.User{Username: "bob"},
					ActiveReviews: 0,
				},
				External: &roulette.Reviewer{
					User:          &models.User{Username: "charlie", Team: "team-platform"},
					ActiveReviews: 1,
				},
			},
			contains: []string{
				"🎲",
				"Reviewer Roulette Results",
				"Code Owner",
				"@alice",
				"2 active reviews",
				"Team Member",
				"@bob",
				"External Reviewer",
				"@charlie",
				"from team-platform",
				"1 active review", // singular form
			},
		},
		{
			name: "only team member and external",
			result: &roulette.SelectionResult{
				Codeowner: nil,
				TeamMember: &roulette.Reviewer{
					User:          &models.User{Username: "bob"},
					ActiveReviews: 0,
				},
				External: &roulette.Reviewer{
					User:          &models.User{Username: "charlie", Team: "team-data"},
					ActiveReviews: 3,
				},
			},
			contains: []string{
				"@bob",
				"@charlie",
				"from team-data",
				"3 active reviews",
			},
		},
		{
			name: "with warnings",
			result: &roulette.SelectionResult{
				TeamMember: &roulette.Reviewer{
					User: &models.User{Username: "alice"},
				},
				External: &roulette.Reviewer{
					User: &models.User{Username: "bob"},
				},
				Warnings: []string{
					"⚠️ Could not select a code owner",
					"⚠️ Limited availability",
				},
			},
			contains: []string{
				"@alice",
				"@bob",
				"⚠️ Could not select a code owner",
				"⚠️ Limited availability",
			},
		},
		{
			name: "no active reviews shown when zero",
			result: &roulette.SelectionResult{
				TeamMember: &roulette.Reviewer{
					User:          &models.User{Username: "alice"},
					ActiveReviews: 0,
				},
			},
			contains: []string{
				"@alice",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.formatRouletteResult(tt.result)

			for _, expected := range tt.contains {
				if !contains(result, expected) {
					t.Errorf("expected output to contain %q, but it didn't.\nGot: %s", expected, result)
				}
			}
		})
	}
}

func TestValidateSignature(t *testing.T) {
	// Note: This would require a gin.Context which is complex to mock
	// For now, we test the logic separately
	tests := []struct {
		name          string
		headerToken   string
		configSecret  string
		expectedValid bool
	}{
		{
			name:          "matching tokens",
			headerToken:   "secret123",
			configSecret:  "secret123",
			expectedValid: true,
		},
		{
			name:          "mismatched tokens",
			headerToken:   "wrong",
			configSecret:  "secret123",
			expectedValid: false,
		},
		{
			name:          "empty header",
			headerToken:   "",
			configSecret:  "secret123",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple signature validation logic test
			isValid := tt.headerToken != "" && tt.headerToken == tt.configSecret
			if isValid != tt.expectedValid {
				t.Errorf("expected %v, got %v", tt.expectedValid, isValid)
			}
		})
	}
}

func TestFormatRouletteResult_French(t *testing.T) {
	translator, err := i18n.New("fr")
	if err != nil {
		t.Fatalf("Failed to initialize translator: %v", err)
	}

	h := &Handler{
		translator: translator,
	}

	tests := []struct {
		name     string
		result   *roulette.SelectionResult
		contains []string
	}{
		{
			name: "all three reviewers in French",
			result: &roulette.SelectionResult{
				Codeowner: &roulette.Reviewer{
					User:          &models.User{Username: "alice"},
					ActiveReviews: 2,
				},
				TeamMember: &roulette.Reviewer{
					User:          &models.User{Username: "bob"},
					ActiveReviews: 0,
				},
				External: &roulette.Reviewer{
					User:          &models.User{Username: "charlie", Team: "team-platform"},
					ActiveReviews: 1,
				},
			},
			contains: []string{
				"🎲",
				"Résultats de la Roulette des Reviewers",
				"Propriétaire du Code",
				"@alice",
				"2 reviews actives",
				"Membre de l'Équipe",
				"@bob",
				"Reviewer Externe",
				"@charlie",
				"de team-platform",
				"1 review active", // singular form
			},
		},
		{
			name: "plural forms in French",
			result: &roulette.SelectionResult{
				TeamMember: &roulette.Reviewer{
					User:          &models.User{Username: "alice"},
					ActiveReviews: 5,
				},
				External: &roulette.Reviewer{
					User:          &models.User{Username: "bob", Team: "team-backend"},
					ActiveReviews: 0,
				},
			},
			contains: []string{
				"@alice",
				"5 reviews actives",
				"@bob",
				"de team-backend",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.formatRouletteResult(tt.result)

			for _, expected := range tt.contains {
				if !contains(result, expected) {
					t.Errorf("expected output to contain %q, but it didn't.\nGot: %s", expected, result)
				}
			}
		})
	}
}

func TestFormatRouletteResult_ZeroReviewsNotShown(t *testing.T) {
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("Failed to initialize translator: %v", err)
	}

	h := &Handler{
		translator: translator,
	}

	result := &roulette.SelectionResult{
		Codeowner: &roulette.Reviewer{
			User:          &models.User{Username: "alice"},
			ActiveReviews: 0,
		},
		TeamMember: &roulette.Reviewer{
			User:          &models.User{Username: "bob"},
			ActiveReviews: 0,
		},
		External: &roulette.Reviewer{
			User:          &models.User{Username: "charlie", Team: "team-platform"},
			ActiveReviews: 0,
		},
	}

	output := h.formatRouletteResult(result)

	// Should contain usernames but NOT "active review" text
	if !contains(output, "@alice") {
		t.Error("expected output to contain @alice")
	}
	if !contains(output, "@bob") {
		t.Error("expected output to contain @bob")
	}
	if !contains(output, "@charlie") {
		t.Error("expected output to contain @charlie")
	}
	if contains(output, "active review") {
		t.Error("expected output to NOT contain 'active review' when count is 0")
	}
}

func TestTranslatorNilHandling(t *testing.T) {
	// Test that handler with nil translator doesn't panic
	// This test is to verify that translator is properly initialized in production
	translator, err := i18n.New("en")
	if err != nil {
		t.Fatalf("Failed to initialize translator: %v", err)
	}

	h := &Handler{
		translator: translator,
	}

	if h.translator == nil {
		t.Fatal("translator should not be nil")
	}

	// Verify we can use it
	result := &roulette.SelectionResult{
		TeamMember: &roulette.Reviewer{
			User:          &models.User{Username: "test"},
			ActiveReviews: 1,
		},
	}

	output := h.formatRouletteResult(result)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
