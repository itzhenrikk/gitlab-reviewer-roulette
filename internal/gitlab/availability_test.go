package gitlab

import (
	"testing"
)

func TestIsUserAvailable(t *testing.T) {
	oooKeywords := []string{"vacation", "ooo", "out of office", "pto", "holiday"}

	tests := []struct {
		name     string
		status   *UserStatus
		keywords []string
		expected bool
	}{
		{
			name:     "nil status means available",
			status:   nil,
			keywords: oooKeywords,
			expected: true,
		},
		{
			name: "busy status means unavailable",
			status: &UserStatus{
				Availability: "busy",
				Message:      "",
			},
			keywords: oooKeywords,
			expected: false,
		},
		{
			name: "empty status means available",
			status: &UserStatus{
				Availability: "",
				Message:      "",
			},
			keywords: oooKeywords,
			expected: true,
		},
		{
			name: "ooo keyword in message",
			status: &UserStatus{
				Availability: "",
				Message:      "OOO until Friday",
			},
			keywords: oooKeywords,
			expected: false,
		},
		{
			name: "vacation keyword in message",
			status: &UserStatus{
				Availability: "",
				Message:      "On vacation",
			},
			keywords: oooKeywords,
			expected: false,
		},
		{
			name: "pto keyword case insensitive",
			status: &UserStatus{
				Availability: "",
				Message:      "Taking PTO today",
			},
			keywords: oooKeywords,
			expected: false,
		},
		{
			name: "holiday keyword in French",
			status: &UserStatus{
				Availability: "",
				Message:      "En holiday cette semaine",
			},
			keywords: oooKeywords,
			expected: false,
		},
		{
			name: "normal status message",
			status: &UserStatus{
				Availability: "",
				Message:      "Working on feature X",
			},
			keywords: oooKeywords,
			expected: true,
		},
		{
			name: "busy with ooo message",
			status: &UserStatus{
				Availability: "busy",
				Message:      "OOO",
			},
			keywords: oooKeywords,
			expected: false, // busy takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUserAvailable(tt.status, tt.keywords)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
