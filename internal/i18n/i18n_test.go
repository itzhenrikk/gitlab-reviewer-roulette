package i18n

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		lang           string
		expectLang     string
		expectError    bool
		expectFallback bool
	}{
		{
			name:        "English language",
			lang:        "en",
			expectLang:  "en",
			expectError: false,
		},
		{
			name:        "French language",
			lang:        "fr",
			expectLang:  "fr",
			expectError: false,
		},
		{
			name:        "Empty language defaults to English",
			lang:        "",
			expectLang:  "en",
			expectError: false,
		},
		{
			name:           "Invalid language falls back to English",
			lang:           "es",
			expectLang:     "en",
			expectError:    false,
			expectFallback: true,
		},
		{
			name:           "Unknown language falls back to English",
			lang:           "invalid",
			expectLang:     "en",
			expectError:    false,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator, err := New(tt.lang)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if translator == nil {
				t.Fatal("translator is nil")
			}

			if translator.Lang() != tt.expectLang {
				t.Errorf("expected language %q, got %q", tt.expectLang, translator.Lang())
			}

			// Verify translator has messages loaded
			if len(translator.messages) == 0 {
				t.Error("translator has no messages loaded")
			}
		})
	}
}

func TestGet(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		key      string
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "Simple key without template",
			key:      "roulette.title",
			data:     nil,
			expected: "🎲 Reviewer Roulette Results",
		},
		{
			name:     "Key with template variables",
			key:      "roulette.from_team",
			data:     map[string]interface{}{"Team": "team-frontend"},
			expected: "from team-frontend",
		},
		{
			name:     "Non-existent key returns key itself",
			key:      "nonexistent.key",
			data:     nil,
			expected: "nonexistent.key",
		},
		{
			name:     "Error message with template",
			key:      "errors.selection_failed",
			data:     map[string]interface{}{"Error": "no reviewers found"},
			expected: "❌ Roulette Error\n\nFailed to select reviewers: no reviewers found",
		},
		{
			name:     "Key without data when template exists",
			key:      "roulette.from_team",
			data:     nil,
			expected: "from {{.Team}}", // Should return raw template
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.data != nil {
				result = translator.Get(tt.key, tt.data)
			} else {
				result = translator.Get(tt.key)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetWithFallback(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		key      string
		fallback string
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "Existing key returns translation",
			key:      "roulette.title",
			fallback: "Fallback Title",
			data:     nil,
			expected: "🎲 Reviewer Roulette Results",
		},
		{
			name:     "Non-existent key returns fallback",
			key:      "nonexistent.key",
			fallback: "Custom Fallback",
			data:     nil,
			expected: "Custom Fallback",
		},
		{
			name:     "Existing key with template data",
			key:      "roulette.from_team",
			fallback: "Fallback",
			data:     map[string]interface{}{"Team": "team-backend"},
			expected: "from team-backend",
		},
		{
			name:     "Non-existent key with data returns fallback",
			key:      "missing.key",
			fallback: "Default Value",
			data:     map[string]interface{}{"Foo": "bar"},
			expected: "Default Value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.data != nil {
				result = translator.GetWithFallback(tt.key, tt.fallback, tt.data)
			} else {
				result = translator.GetWithFallback(tt.key, tt.fallback)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetPlural(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		key      string
		count    int
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "Singular form (count=1)",
			key:      "roulette.active_reviews",
			count:    1,
			data:     nil,
			expected: "1 active review",
		},
		{
			name:     "Plural form (count=0)",
			key:      "roulette.active_reviews",
			count:    0,
			data:     nil,
			expected: "0 active reviews",
		},
		{
			name:     "Plural form (count=2)",
			key:      "roulette.active_reviews",
			count:    2,
			data:     nil,
			expected: "2 active reviews",
		},
		{
			name:     "Plural form (count=5)",
			key:      "roulette.active_reviews",
			count:    5,
			data:     nil,
			expected: "5 active reviews",
		},
		{
			name:     "Plural with additional data",
			key:      "roulette.active_reviews",
			count:    3,
			data:     map[string]interface{}{"Extra": "value"},
			expected: "3 active reviews",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.data != nil {
				result = translator.GetPlural(tt.key, tt.count, tt.data)
			} else {
				result = translator.GetPlural(tt.key, tt.count)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetPlural_French(t *testing.T) {
	translator, err := New("fr")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "French singular (count=1)",
			count:    1,
			expected: "1 review active",
		},
		{
			name:     "French plural (count=0)",
			count:    0,
			expected: "0 reviews actives",
		},
		{
			name:     "French plural (count=2)",
			count:    2,
			expected: "2 reviews actives",
		},
		{
			name:     "French plural (count=5)",
			count:    5,
			expected: "5 reviews actives",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.GetPlural("roulette.active_reviews", tt.count)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLang(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		expected string
	}{
		{
			name:     "English",
			lang:     "en",
			expected: "en",
		},
		{
			name:     "French",
			lang:     "fr",
			expected: "fr",
		},
		{
			name:     "Empty defaults to English",
			lang:     "",
			expected: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator, err := New(tt.lang)
			if err != nil {
				t.Fatalf("Failed to create translator: %v", err)
			}

			if translator.Lang() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, translator.Lang())
			}
		})
	}
}

func TestActiveReviewsMessage(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "Zero returns empty string",
			count:    0,
			expected: "",
		},
		{
			name:     "One returns singular",
			count:    1,
			expected: "1 active review",
		},
		{
			name:     "Multiple returns plural",
			count:    3,
			expected: "3 active reviews",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.ActiveReviewsMessage(tt.count)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatActiveReviews(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "Zero returns empty string",
			count:    0,
			expected: "",
		},
		{
			name:     "One returns formatted singular",
			count:    1,
			expected: " (1 active review)",
		},
		{
			name:     "Multiple returns formatted plural",
			count:    3,
			expected: " (3 active reviews)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.FormatActiveReviews(tt.count)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatActiveReviews_French(t *testing.T) {
	translator, err := New("fr")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "Zero returns empty string",
			count:    0,
			expected: "",
		},
		{
			name:     "One returns formatted singular",
			count:    1,
			expected: " (1 review active)",
		},
		{
			name:     "Multiple returns formatted plural",
			count:    3,
			expected: " (3 reviews actives)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.FormatActiveReviews(tt.count)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFromTeamMessage(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		teamName string
		expected string
	}{
		{
			name:     "Simple team name",
			teamName: "team-frontend",
			expected: "from team-frontend",
		},
		{
			name:     "Team with hyphens",
			teamName: "team-backend-api",
			expected: "from team-backend-api",
		},
		{
			name:     "Empty team name",
			teamName: "",
			expected: "from ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.FromTeamMessage(tt.teamName)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFromTeamMessage_French(t *testing.T) {
	translator, err := New("fr")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		teamName string
		expected string
	}{
		{
			name:     "Simple team name",
			teamName: "team-frontend",
			expected: "de team-frontend",
		},
		{
			name:     "Team with hyphens",
			teamName: "team-backend-api",
			expected: "de team-backend-api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.FromTeamMessage(tt.teamName)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTitleWithNewlines(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	result := translator.TitleWithNewlines()
	expected := "🎲 Reviewer Roulette Results\n\n"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTitleWithNewlines_French(t *testing.T) {
	translator, err := New("fr")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	result := translator.TitleWithNewlines()
	expected := "🎲 Résultats de la Roulette des Reviewers\n\n"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAllEnglishTranslations(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	// Verify all expected keys exist
	expectedKeys := []string{
		"roulette.title",
		"roulette.codeowner",
		"roulette.team_member",
		"roulette.external",
		"roulette.active_reviews",
		"roulette.active_reviews_plural",
		"roulette.from_team",
		"warnings.no_team_label",
		"warnings.no_codeowners",
		"warnings.limited_availability",
		"warnings.no_team_members",
		"warnings.no_external_reviewers",
		"errors.selection_failed",
		"errors.invalid_command",
		"errors.webhook_processing",
	}

	for _, key := range expectedKeys {
		t.Run("key_"+key, func(t *testing.T) {
			result := translator.Get(key)
			if result == key {
				t.Errorf("translation missing for key %q", key)
			}
			if result == "" {
				t.Errorf("translation is empty for key %q", key)
			}
		})
	}
}

func TestAllFrenchTranslations(t *testing.T) {
	translator, err := New("fr")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	// Verify all expected keys exist
	expectedKeys := []string{
		"roulette.title",
		"roulette.codeowner",
		"roulette.team_member",
		"roulette.external",
		"roulette.active_reviews",
		"roulette.active_reviews_plural",
		"roulette.from_team",
		"warnings.no_team_label",
		"warnings.no_codeowners",
		"warnings.limited_availability",
		"warnings.no_team_members",
		"warnings.no_external_reviewers",
		"errors.selection_failed",
		"errors.invalid_command",
		"errors.webhook_processing",
	}

	for _, key := range expectedKeys {
		t.Run("key_"+key, func(t *testing.T) {
			result := translator.Get(key)
			if result == key {
				t.Errorf("translation missing for key %q", key)
			}
			if result == "" {
				t.Errorf("translation is empty for key %q", key)
			}
		})
	}
}

func TestTemplateErrorHandling(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		key      string
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "Missing template variable returns <no value>",
			key:      "roulette.from_team",
			data:     map[string]interface{}{},
			expected: "from <no value>", // Template executed with missing variable
		},
		{
			name:     "Wrong template variable type",
			key:      "roulette.from_team",
			data:     map[string]interface{}{"Team": 123}, // Number instead of string
			expected: "from 123",                          // Should convert to string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.Get(tt.key, tt.data)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	translator, err := New("en")
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}

	// Test concurrent reads (should be safe since we're only reading)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = translator.Get("roulette.title")
				_ = translator.GetPlural("roulette.active_reviews", j%5)
				_ = translator.FormatActiveReviews(j % 3)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFlattenMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name: "Simple flat map",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Nested map",
			input: map[string]interface{}{
				"section": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: map[string]string{
				"section.key1": "value1",
				"section.key2": "value2",
			},
		},
		{
			name: "Deeply nested map",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"key": "value",
					},
				},
			},
			expected: map[string]string{
				"level1.level2.key": "value",
			},
		},
		{
			name: "Mixed types",
			input: map[string]interface{}{
				"string": "text",
				"number": 42,
				"nested": map[string]interface{}{
					"bool": true,
				},
			},
			expected: map[string]string{
				"string":      "text",
				"number":      "42",
				"nested.bool": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]string)
			flattenMap(tt.input, "", result)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("for key %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}
