package roulette

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/cache"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/gitlab"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// Service handles reviewer selection logic.
type Service struct {
	config       *config.Config
	gitlabClient *gitlab.Client
	userRepo     *repository.UserRepository
	oooRepo      *repository.OOORepository
	reviewRepo   *repository.ReviewRepository
	cache        *cache.Cache
	log          *logger.Logger
}

// NewService creates a new roulette service.
func NewService(
	cfg *config.Config,
	gitlabClient *gitlab.Client,
	userRepo *repository.UserRepository,
	oooRepo *repository.OOORepository,
	reviewRepo *repository.ReviewRepository,
	cacheClient *cache.Cache,
	log *logger.Logger,
) *Service {
	return &Service{
		config:       cfg,
		gitlabClient: gitlabClient,
		userRepo:     userRepo,
		oooRepo:      oooRepo,
		reviewRepo:   reviewRepo,
		cache:        cacheClient,
		log:          log,
	}
}

// SelectionRequest represents a reviewer selection request.
type SelectionRequest struct {
	ProjectID int
	MRIID     int
	TriggerBy string
	Options   SelectionOptions
}

// SelectionOptions contains options for reviewer selection.
type SelectionOptions struct {
	Force        bool     // Override recent review penalties
	IncludeUsers []string // Force include specific users
	ExcludeUsers []string // Exclude specific users
	NoCodeowner  bool     // Skip codeowner selection
}

// SelectionResult represents the result of reviewer selection.
type SelectionResult struct {
	Codeowner  *Reviewer
	TeamMember *Reviewer
	External   *Reviewer
	Warnings   []string
	Team       string
	Role       string
}

// Reviewer represents a selected reviewer.
type Reviewer struct {
	User          *models.User
	ActiveReviews int
	Score         float64
}

// SelectReviewers performs the reviewer selection algorithm.
func (s *Service) SelectReviewers(ctx context.Context, req *SelectionRequest) (*SelectionResult, error) {
	s.log.Info().
		Int("project_id", req.ProjectID).
		Int("mr_iid", req.MRIID).
		Msg("Starting reviewer selection")

	// Get MR details
	mr, err := s.gitlabClient.GetMergeRequest(req.ProjectID, req.MRIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR: %w", err)
	}

	result := &SelectionResult{
		Warnings: make([]string, 0),
	}

	// 1. Parse MR context (team label, role label)
	team, role := s.extractTeamAndRole(mr.Labels)
	result.Team = team
	result.Role = role

	if team == "" {
		result.Warnings = append(result.Warnings, "⚠️ No team label found. Please add a `name::team-name` label to this MR.")
	}

	// 2. Get modified files
	changes, err := s.gitlabClient.GetMergeRequestChanges(req.ProjectID, req.MRIID)
	if err != nil {
		s.log.Warn().Err(err).Msg("Failed to get MR changes")
		changes = nil
	}

	modifiedFiles := make([]string, 0)
	for _, change := range changes {
		modifiedFiles = append(modifiedFiles, change.NewPath)
	}

	// 3. Select codeowner (if not skipped)
	if !req.Options.NoCodeowner {
		codeowner, err := s.selectCodeowner(ctx, req, modifiedFiles)
		if err != nil {
			s.log.Warn().Err(err).Msg("Failed to select codeowner")
			result.Warnings = append(result.Warnings, "⚠️ Could not select a code owner. CODEOWNERS file may be missing or no owners are available.")
		} else {
			result.Codeowner = codeowner
		}
	}

	// 4. Select team member
	if team != "" {
		teamMember, err := s.selectTeamMember(ctx, req, team, role, result.Codeowner, modifiedFiles)
		if err != nil {
			s.log.Warn().Err(err).Msg("Failed to select team member")
			result.Warnings = append(result.Warnings, "⚠️ Could not select a team member. All team members may be unavailable.")
		} else {
			result.TeamMember = teamMember
		}
	}

	// 5. Select external reviewer
	external, err := s.selectExternal(ctx, req, team, result.Codeowner, result.TeamMember, modifiedFiles)
	if err != nil {
		s.log.Warn().Err(err).Msg("Failed to select external reviewer")
		result.Warnings = append(result.Warnings, "⚠️ Could not select an external reviewer. All users may be unavailable.")
	} else {
		result.External = external
	}

	s.log.Info().
		Bool("has_codeowner", result.Codeowner != nil).
		Bool("has_team_member", result.TeamMember != nil).
		Bool("has_external", result.External != nil).
		Int("warnings", len(result.Warnings)).
		Msg("Reviewer selection completed")

	return result, nil
}

// extractTeamAndRole extracts team and role from MR labels.
func (s *Service) extractTeamAndRole(labels []string) (string, string) {
	team := ""
	role := ""

	for _, label := range labels {
		// Check for scoped label: name::team-name
		if strings.Contains(label, "::") {
			parts := strings.Split(label, "::")
			if len(parts) == 2 && parts[0] == "name" {
				team = parts[1]
			}
		}

		// Check for role labels
		labelLower := strings.ToLower(label)
		switch labelLower {
		case "dev":
			role = "dev"
		case "ops":
			role = "ops"
		}
	}

	return team, role
}

// selectCodeowner selects a code owner based on modified files.
func (s *Service) selectCodeowner(ctx context.Context, req *SelectionRequest, modifiedFiles []string) (*Reviewer, error) {
	// Get CODEOWNERS file
	content, err := s.gitlabClient.GetCodeowners(req.ProjectID, "main") // or "master"
	if err != nil {
		return nil, fmt.Errorf("failed to get CODEOWNERS: %w", err)
	}

	// Parse CODEOWNERS
	owners := gitlab.ParseCodeowners(content)

	// Find relevant owners for modified files
	relevantOwners := make(map[string]bool)
	for _, file := range modifiedFiles {
		for pattern, ownersList := range owners {
			if matchPattern(pattern, file) {
				for _, owner := range ownersList {
					relevantOwners[owner] = true
				}
			}
		}
	}

	// If no specific owners found and modifiedFiles is empty or no matches, try default pattern "*"
	if len(relevantOwners) == 0 {
		if defaultOwners, exists := owners["*"]; exists {
			for _, owner := range defaultOwners {
				relevantOwners[owner] = true
			}
		}
	}

	if len(relevantOwners) == 0 {
		return nil, fmt.Errorf("no code owners found for modified files")
	}

	// Get users for owners
	candidates := make([]*models.User, 0)
	for owner := range relevantOwners {
		user, err := s.userRepo.GetByUsername(owner)
		if err != nil {
			s.log.Warn().Str("username", owner).Msg("Owner not found in database")
			continue
		}
		candidates = append(candidates, user)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no valid code owners found")
	}

	// Filter by availability and select
	return s.selectBestReviewer(ctx, candidates, req.Options, modifiedFiles)
}

// selectTeamMember selects a team member.
func (s *Service) selectTeamMember(ctx context.Context, req *SelectionRequest, team, role string, exclude *Reviewer, modifiedFiles []string) (*Reviewer, error) {
	// Get team members
	var candidates []models.User
	var err error

	if role != "" {
		candidates, err = s.userRepo.GetByTeamAndRole(team, role)
	} else {
		candidates, err = s.userRepo.GetByTeam(team)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	// Convert to pointers and exclude already selected
	candidatePtrs := make([]*models.User, 0)
	for i := range candidates {
		if exclude != nil && candidates[i].ID == exclude.User.ID {
			continue
		}
		candidatePtrs = append(candidatePtrs, &candidates[i])
	}

	if len(candidatePtrs) == 0 {
		return nil, fmt.Errorf("no team members available")
	}

	return s.selectBestReviewer(ctx, candidatePtrs, req.Options, modifiedFiles)
}

// selectExternal selects an external reviewer (from other teams).
func (s *Service) selectExternal(ctx context.Context, req *SelectionRequest, currentTeam string, exclude1, exclude2 *Reviewer, modifiedFiles []string) (*Reviewer, error) {
	// Get all users
	allUsers, err := s.userRepo.List("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	// Filter: different team and not already selected
	candidates := make([]*models.User, 0)
	for i := range allUsers {
		if allUsers[i].Team == currentTeam {
			continue
		}
		if exclude1 != nil && allUsers[i].ID == exclude1.User.ID {
			continue
		}
		if exclude2 != nil && allUsers[i].ID == exclude2.User.ID {
			continue
		}
		candidates = append(candidates, &allUsers[i])
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no external reviewers available")
	}

	return s.selectBestReviewer(ctx, candidates, req.Options, modifiedFiles)
}

// selectBestReviewer selects the best reviewer from candidates using weighting algorithm.
func (s *Service) selectBestReviewer(ctx context.Context, candidates []*models.User, options SelectionOptions, modifiedFiles []string) (*Reviewer, error) {
	available := make([]*Reviewer, 0)

	for _, user := range candidates {
		// Check if user should be excluded
		if contains(options.ExcludeUsers, user.Username) {
			continue
		}

		// Check availability
		isAvailable, err := s.isUserAvailable(ctx, user)
		if err != nil {
			s.log.Warn().Err(err).Uint("user_id", user.ID).Msg("Failed to check availability")
			continue
		}

		if !isAvailable {
			continue
		}

		// Calculate score (now with expertise matching)
		score := s.calculateScore(ctx, user, options, modifiedFiles)

		// Get active reviews count (with caching)
		activeReviews := s.getActiveReviewsCount(ctx, user.ID)

		available = append(available, &Reviewer{
			User:          user,
			ActiveReviews: activeReviews,
			Score:         score,
		})
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no available reviewers")
	}

	// Check force include
	for _, username := range options.IncludeUsers {
		for _, reviewer := range available {
			if reviewer.User.Username == username {
				return reviewer, nil
			}
		}
	}

	// Select highest scoring reviewer (with some randomness for equal scores)
	return selectByScore(available), nil
}

// calculateScore calculates a reviewer's score based on weighting algorithm.
func (s *Service) calculateScore(ctx context.Context, user *models.User, options SelectionOptions, modifiedFiles []string) float64 {
	score := 100.0

	// Penalty for current load (with caching)
	activeReviews := s.getActiveReviewsCount(ctx, user.ID)
	score -= float64(activeReviews) * float64(s.config.Roulette.Weights.CurrentLoad)

	// Penalty for recent reviews (unless force option)
	if !options.Force {
		since := time.Now().Add(-24 * time.Hour)
		recentAssignments, _ := s.reviewRepo.GetRecentAssignmentsByUserID(user.ID, since)
		if len(recentAssignments) > 0 {
			score -= float64(s.config.Roulette.Weights.RecentReview)
		}
	}

	// Expertise bonus based on file types (Phase 2)
	if s.hasExpertise(user.Role, modifiedFiles) {
		score += float64(s.config.Roulette.Weights.ExpertiseBonus)
		s.log.Debug().
			Str("username", user.Username).
			Str("role", user.Role).
			Int("files", len(modifiedFiles)).
			Msg("Applied expertise bonus")
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// hasExpertise checks if user has expertise for the modified files.
func (s *Service) hasExpertise(role string, modifiedFiles []string) bool {
	if len(modifiedFiles) == 0 {
		return false
	}

	var expertisePatterns []string
	switch role {
	case "dev":
		expertisePatterns = s.config.Roulette.Expertise.Dev
	case "ops":
		expertisePatterns = s.config.Roulette.Expertise.Ops
	default:
		return false
	}

	// Check if any modified file matches the role's expertise patterns
	for _, file := range modifiedFiles {
		for _, pattern := range expertisePatterns {
			if matchPattern(pattern, filepath.Base(file)) {
				return true
			}
		}
	}

	return false
}

// getActiveReviewsCount gets user's active review count with Redis caching.
func (s *Service) getActiveReviewsCount(ctx context.Context, userID uint) int {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("user:review_count:%d", userID)
	cachedValue, err := s.cache.Get(ctx, cacheKey)
	if err == nil && cachedValue != "" {
		// Parse cached count
		var count int
		if _, err := fmt.Sscanf(cachedValue, "%d", &count); err == nil {
			s.log.Debug().Uint("user_id", userID).Int("count", count).Msg("Using cached review count")
			return count
		}
	}

	// Fetch from database
	count, err := s.reviewRepo.CountActiveReviewsByUserID(userID)
	if err != nil {
		s.log.Warn().Err(err).Uint("user_id", userID).Msg("Failed to get active reviews count")
		return 0
	}

	// Cache for 5 minutes (use same TTL as availability)
	_ = s.cache.Set(ctx, cacheKey, fmt.Sprintf("%d", count), time.Duration(s.config.Availability.CacheTTL)*time.Second)

	s.log.Debug().
		Uint("user_id", userID).
		Int64("count", count).
		Msg("Cached review count")

	return int(count)
}

// isUserAvailable checks if a user is available for review (with Redis caching).
func (s *Service) isUserAvailable(ctx context.Context, user *models.User) (bool, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("user:availability:%d", user.ID)
	cachedValue, err := s.cache.Get(ctx, cacheKey)
	if err == nil && cachedValue != "" {
		s.log.Debug().Uint("user_id", user.ID).Msg("Using cached availability")
		return cachedValue == "available", nil
	}

	// Check OOO database
	isOOO, err := s.oooRepo.IsUserOOO(user.ID)
	if err != nil {
		return false, err
	}
	if isOOO {
		// Cache for 5 minutes
		_ = s.cache.Set(ctx, cacheKey, "unavailable", time.Duration(s.config.Availability.CacheTTL)*time.Second)
		return false, nil
	}

	// Check GitLab status
	status, err := s.gitlabClient.GetUserStatus(user.GitLabID)
	if err != nil {
		s.log.Warn().Err(err).Int("gitlab_id", user.GitLabID).Msg("Failed to get user status")
		// If we can't get status, assume available (don't cache errors)
		return true, nil
	}

	isAvailable := gitlab.IsUserAvailable(status, s.config.Availability.OOOKeywords)

	// Cache the result for 5 minutes
	availabilityStr := "available"
	if !isAvailable {
		availabilityStr = "unavailable"
	}
	_ = s.cache.Set(ctx, cacheKey, availabilityStr, time.Duration(s.config.Availability.CacheTTL)*time.Second)

	s.log.Debug().
		Uint("user_id", user.ID).
		Bool("available", isAvailable).
		Msg("Cached availability result")

	return isAvailable, nil
}

// Helper functions

func matchPattern(pattern, file string) bool {
	matched, _ := filepath.Match(pattern, file)
	return matched
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func selectByScore(reviewers []*Reviewer) *Reviewer {
	if len(reviewers) == 0 {
		return nil
	}

	// Find max score
	maxScore := reviewers[0].Score
	for _, r := range reviewers {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	// Get all reviewers with max score
	topReviewers := make([]*Reviewer, 0)
	for _, r := range reviewers {
		if r.Score == maxScore {
			topReviewers = append(topReviewers, r)
		}
	}

	// Random selection among top scorers (rand is automatically seeded in Go 1.20+)
	return topReviewers[rand.Intn(len(topReviewers))]
}
