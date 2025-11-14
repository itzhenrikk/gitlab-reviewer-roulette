package roulette

import (
	"testing"
)

func TestExtractTeamAndRole(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name         string
		labels       []string
		expectedTeam string
		expectedRole string
	}{
		{
			name:         "team and dev role",
			labels:       []string{"name::team-frontend", "dev"},
			expectedTeam: "team-frontend",
			expectedRole: "dev",
		},
		{
			name:         "team and ops role",
			labels:       []string{"name::team-platform", "ops"},
			expectedTeam: "team-platform",
			expectedRole: "ops",
		},
		{
			name:         "team only",
			labels:       []string{"name::team-backend"},
			expectedTeam: "team-backend",
			expectedRole: "",
		},
		{
			name:         "role only",
			labels:       []string{"dev"},
			expectedTeam: "",
			expectedRole: "dev",
		},
		{
			name:         "case insensitive role",
			labels:       []string{"DEV"},
			expectedTeam: "",
			expectedRole: "dev",
		},
		{
			name:         "ops case insensitive",
			labels:       []string{"OPS"},
			expectedTeam: "",
			expectedRole: "ops",
		},
		{
			name:         "no team or role",
			labels:       []string{"bug", "priority::high"},
			expectedTeam: "",
			expectedRole: "",
		},
		{
			name:         "multiple labels with team and role",
			labels:       []string{"bug", "name::team-mobile", "dev", "priority::high"},
			expectedTeam: "team-mobile",
			expectedRole: "dev",
		},
		{
			name:         "empty labels",
			labels:       []string{},
			expectedTeam: "",
			expectedRole: "",
		},
		{
			name:         "wrong scoped label format",
			labels:       []string{"priority::high", "status::review"},
			expectedTeam: "",
			expectedRole: "",
		},
		{
			name:         "name scoped but wrong format",
			labels:       []string{"name::"},
			expectedTeam: "",
			expectedRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team, role := s.extractTeamAndRole(tt.labels)

			if team != tt.expectedTeam {
				t.Errorf("expected team %q, got %q", tt.expectedTeam, team)
			}

			if role != tt.expectedRole {
				t.Errorf("expected role %q, got %q", tt.expectedRole, role)
			}
		})
	}
}
