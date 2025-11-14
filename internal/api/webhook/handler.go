// Package webhook handles GitLab webhook events for the reviewer roulette system.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/gitlab"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/i18n"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/mattermost"
	prommetrics "github.com/aimd54/gitlab-reviewer-roulette/internal/metrics"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/metrics"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/roulette"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// Handler handles GitLab webhook events
type Handler struct {
	config           *config.Config
	gitlabClient     *gitlab.Client
	mattermostClient *mattermost.Client
	rouletteService  *roulette.Service
	metricsService   *metrics.Service
	userRepo         *repository.UserRepository
	reviewRepo       *repository.ReviewRepository
	translator       *i18n.Translator
	log              *logger.Logger
}

// NewHandler creates a new webhook handler
func NewHandler(
	cfg *config.Config,
	gitlabClient *gitlab.Client,
	mattermostClient *mattermost.Client,
	rouletteService *roulette.Service,
	metricsService *metrics.Service,
	userRepo *repository.UserRepository,
	reviewRepo *repository.ReviewRepository,
	translator *i18n.Translator,
	log *logger.Logger,
) *Handler {
	return &Handler{
		config:           cfg,
		gitlabClient:     gitlabClient,
		mattermostClient: mattermostClient,
		rouletteService:  rouletteService,
		metricsService:   metricsService,
		userRepo:         userRepo,
		reviewRepo:       reviewRepo,
		translator:       translator,
		log:              log,
	}
}

// HandleGitLabWebhook processes GitLab webhook events
func (h *Handler) HandleGitLabWebhook(c *gin.Context) {
	// Validate webhook signature
	if !h.validateSignature(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Get event type
	eventType := c.GetHeader("X-Gitlab-Event")

	h.log.Debug().
		Str("event_type", eventType).
		Msg("Received GitLab webhook")

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Handle different event types
	switch eventType {
	case "Note Hook":
		h.handleNoteEvent(c, body)
	case "Merge Request Hook":
		h.handleMergeRequestEvent(c, body)
	default:
		h.log.Debug().Str("event_type", eventType).Msg("Unhandled event type")
		c.JSON(http.StatusOK, gin.H{"message": "event type not handled"})
	}
}

// validateSignature validates the webhook signature
func (h *Handler) validateSignature(c *gin.Context) bool {
	signature := c.GetHeader("X-Gitlab-Token")
	if signature == "" {
		h.log.Warn().Msg("Missing X-Gitlab-Token header")
		return false
	}

	return signature == h.config.GitLab.WebhookSecret
}

// NoteEvent represents a GitLab note (comment) event
type NoteEvent struct {
	ObjectKind string `json:"object_kind"`
	User       struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	ProjectID int `json:"project_id"`
	Project   struct {
		ID int `json:"id"`
	} `json:"project"`
	ObjectAttributes struct {
		ID           int    `json:"id"`
		Note         string `json:"note"`
		NoteableType string `json:"noteable_type"`
		NoteableID   int    `json:"noteable_id"`
	} `json:"object_attributes"`
	MergeRequest struct {
		IID   int    `json:"iid"`
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"merge_request"`
}

// handleNoteEvent handles comment events
func (h *Handler) handleNoteEvent(c *gin.Context, body []byte) {
	var event NoteEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.log.Error().Err(err).Msg("Failed to unmarshal note event")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Only process comments on merge requests
	if event.ObjectAttributes.NoteableType != "MergeRequest" {
		c.JSON(http.StatusOK, gin.H{"message": "not a merge request comment"})
		return
	}

	// Check if comment contains /roulette command
	command, options := h.parseRouletteCommand(event.ObjectAttributes.Note)
	if command == "" {
		c.JSON(http.StatusOK, gin.H{"message": "no roulette command found"})
		return
	}

	h.log.Info().
		Int("project_id", event.ProjectID).
		Int("mr_iid", event.MergeRequest.IID).
		Str("username", event.User.Username).
		Msg("Processing roulette command")

	// Process in background to avoid timeout
	go h.processRouletteCommand(context.Background(), event, options)

	c.JSON(http.StatusOK, gin.H{"message": "processing roulette request"})
}

// parseRouletteCommand parses a /roulette command and its options
func (h *Handler) parseRouletteCommand(comment string) (string, roulette.SelectionOptions) {
	// Match /roulette with optional flags
	re := regexp.MustCompile(`(?m)^/roulette(\s+.*)?$`)
	matches := re.FindStringSubmatch(comment)

	if len(matches) == 0 {
		return "", roulette.SelectionOptions{}
	}

	options := roulette.SelectionOptions{}

	if len(matches) > 1 && matches[1] != "" {
		flags := strings.Fields(matches[1])
		for i := 0; i < len(flags); i++ {
			switch flags[i] {
			case "--force":
				options.Force = true
			case "--no-codeowner":
				options.NoCodeowner = true
			case "--include":
				// Next flags are usernames until we hit another flag
				i++
				for i < len(flags) && !strings.HasPrefix(flags[i], "--") {
					username := strings.TrimPrefix(flags[i], "@")
					options.IncludeUsers = append(options.IncludeUsers, username)
					i++
				}
				i-- // Back up one since loop will increment
			case "--exclude":
				i++
				for i < len(flags) && !strings.HasPrefix(flags[i], "--") {
					username := strings.TrimPrefix(flags[i], "@")
					options.ExcludeUsers = append(options.ExcludeUsers, username)
					i++
				}
				i--
			}
		}
	}

	return "roulette", options
}

// processRouletteCommand executes the roulette selection
func (h *Handler) processRouletteCommand(ctx context.Context, event NoteEvent, options roulette.SelectionOptions) {
	// Get or create user
	user, err := h.getOrCreateUser(event.User.ID, event.User.Username)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to get/create user")
		return
	}

	// Execute roulette selection
	req := &roulette.SelectionRequest{
		ProjectID: event.ProjectID,
		MRIID:     event.MergeRequest.IID,
		TriggerBy: event.User.Username,
		Options:   options,
	}

	result, err := h.rouletteService.SelectReviewers(ctx, req)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to select reviewers")
		h.postErrorComment(event.ProjectID, event.MergeRequest.IID, err)
		return
	}

	// Save to database
	mrReview, err := h.saveRouletteResult(event, user, result)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to save roulette result")
		return
	}

	// Record metrics: review triggered
	if h.metricsService != nil && mrReview != nil {
		if err := h.metricsService.RecordReviewTriggered(ctx, mrReview); err != nil {
			h.log.Error().Err(err).Msg("Failed to record review triggered metric")
		}
	}

	// Record Prometheus metrics: roulette triggered
	prommetrics.RecordRouletteTrigger(result.Team, "success")

	// Post or update result to MR
	if err := h.postRouletteResult(event, result, mrReview); err != nil {
		h.log.Error().Err(err).Msg("Failed to post roulette result")
	}
}

// saveRouletteResult saves the roulette result to database and returns the MRReview
func (h *Handler) saveRouletteResult(event NoteEvent, user *models.User, result *roulette.SelectionResult) (*models.MRReview, error) {
	now := time.Now()

	// Create or update MR review
	mrReview := &models.MRReview{
		GitLabMRIID:         event.MergeRequest.IID,
		GitLabProjectID:     event.ProjectID,
		MRURL:               event.MergeRequest.URL,
		MRTitle:             event.MergeRequest.Title,
		Team:                result.Team,
		RouletteTriggeredAt: &now,
		RouletteTriggeredBy: &user.ID,
		Status:              models.MRStatusPending,
	}

	if err := h.reviewRepo.CreateOrUpdateMRReview(mrReview); err != nil {
		return nil, fmt.Errorf("failed to save MR review: %w", err)
	}

	// Delete old assignments
	_ = h.reviewRepo.DeleteAssignmentsByMRReviewID(mrReview.ID)

	// Create assignments
	assignments := make([]*models.ReviewerAssignment, 0)

	if result.Codeowner != nil {
		assignments = append(assignments, &models.ReviewerAssignment{
			MRReviewID: mrReview.ID,
			UserID:     result.Codeowner.User.ID,
			Role:       models.ReviewerRoleCodeowner,
			AssignedAt: now,
		})
	}

	if result.TeamMember != nil {
		assignments = append(assignments, &models.ReviewerAssignment{
			MRReviewID: mrReview.ID,
			UserID:     result.TeamMember.User.ID,
			Role:       models.ReviewerRoleTeamMember,
			AssignedAt: now,
		})
	}

	if result.External != nil {
		assignments = append(assignments, &models.ReviewerAssignment{
			MRReviewID: mrReview.ID,
			UserID:     result.External.User.ID,
			Role:       models.ReviewerRoleExternal,
			AssignedAt: now,
		})
	}

	for _, assignment := range assignments {
		if err := h.reviewRepo.CreateAssignment(assignment); err != nil {
			h.log.Error().Err(err).Msg("Failed to create assignment")
		}
	}

	return mrReview, nil
}

// postRouletteResult posts or updates the selection result as a comment
func (h *Handler) postRouletteResult(event NoteEvent, result *roulette.SelectionResult, mrReview *models.MRReview) error {
	comment := h.formatRouletteResult(result)

	// If we have an existing bot comment, update it; otherwise create new one
	if mrReview.BotCommentID != nil && *mrReview.BotCommentID > 0 {
		err := h.gitlabClient.UpdateComment(event.ProjectID, event.MergeRequest.IID, *mrReview.BotCommentID, comment)
		if err != nil {
			// If update fails (e.g., comment was deleted), create a new one
			h.log.Warn().
				Err(err).
				Int("note_id", *mrReview.BotCommentID).
				Msg("Failed to update existing comment, creating new one")

			noteID, err := h.gitlabClient.PostComment(event.ProjectID, event.MergeRequest.IID, comment)
			if err != nil {
				return err
			}

			// Update the stored comment ID
			mrReview.BotCommentID = &noteID
			_ = h.reviewRepo.UpdateMRReview(mrReview)
		}
		return nil
	}

	// Create new comment
	noteID, err := h.gitlabClient.PostComment(event.ProjectID, event.MergeRequest.IID, comment)
	if err != nil {
		return err
	}

	// Save the comment ID for future updates
	mrReview.BotCommentID = &noteID
	return h.reviewRepo.UpdateMRReview(mrReview)
}

// formatRouletteResult formats the selection result as markdown
func (h *Handler) formatRouletteResult(result *roulette.SelectionResult) string {
	var sb strings.Builder

	// Title
	sb.WriteString(h.translator.TitleWithNewlines())

	// Code Owner
	if result.Codeowner != nil {
		label := h.translator.Get("roulette.codeowner")
		activeReviews := h.translator.FormatActiveReviews(result.Codeowner.ActiveReviews)
		sb.WriteString(fmt.Sprintf("* **%s**: @%s%s\n", label, result.Codeowner.User.Username, activeReviews))
	}

	// Team Member
	if result.TeamMember != nil {
		label := h.translator.Get("roulette.team_member")
		activeReviews := h.translator.FormatActiveReviews(result.TeamMember.ActiveReviews)
		sb.WriteString(fmt.Sprintf("* **%s**: @%s%s\n", label, result.TeamMember.User.Username, activeReviews))
	}

	// External Reviewer
	if result.External != nil {
		label := h.translator.Get("roulette.external")
		activeReviews := h.translator.FormatActiveReviews(result.External.ActiveReviews)
		team := ""
		if result.External.User.Team != "" {
			team = " " + h.translator.FromTeamMessage(result.External.User.Team)
		}
		sb.WriteString(fmt.Sprintf("* **%s**: @%s%s%s\n", label, result.External.User.Username, team, activeReviews))
	}

	// Warnings (add extra blank line for separation)
	if len(result.Warnings) > 0 {
		sb.WriteString("\n") // Add blank line separator
		for _, warning := range result.Warnings {
			sb.WriteString(warning + "\n\n") // Add blank line after each warning
		}
	}

	return sb.String()
}

// postErrorComment posts an error comment
func (h *Handler) postErrorComment(projectID, mrIID int, err error) {
	comment := h.translator.Get("errors.selection_failed", map[string]interface{}{
		"Error": err.Error(),
	})
	_, postErr := h.gitlabClient.PostComment(projectID, mrIID, comment)
	if postErr != nil {
		h.log.Error().Err(postErr).Msg("Failed to post error comment")
	}
}

// MergeRequestEvent represents a GitLab merge request event
type MergeRequestEvent struct {
	ObjectKind string `json:"object_kind"`
	Project    struct {
		ID int `json:"id"`
	} `json:"project"`
	ObjectAttributes struct {
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
		Action string `json:"action"`
	} `json:"object_attributes"`
}

// handleMergeRequestEvent handles MR lifecycle events
func (h *Handler) handleMergeRequestEvent(c *gin.Context, body []byte) {
	var event MergeRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.log.Error().Err(err).Msg("Failed to unmarshal MR event")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	h.log.Debug().
		Int("project_id", event.Project.ID).
		Int("mr_iid", event.ObjectAttributes.IID).
		Str("action", event.ObjectAttributes.Action).
		Str("state", event.ObjectAttributes.State).
		Msg("Processing MR event")

	// Handle approval, merge, or close events
	switch {
	case event.ObjectAttributes.Action == "approved":
		go h.handleMRApproved(context.Background(), event)
	case event.ObjectAttributes.Action == "merge" || event.ObjectAttributes.State == "merged":
		go h.handleMRMerged(context.Background(), event)
	case event.ObjectAttributes.State == "closed":
		go h.handleMRClosed(context.Background(), event)
	}

	c.JSON(http.StatusOK, gin.H{"message": "processed"})
}

// handleMRMerged updates the review status when MR is merged
func (h *Handler) handleMRMerged(_ context.Context, event MergeRequestEvent) {
	review, err := h.reviewRepo.GetMRReview(event.Project.ID, event.ObjectAttributes.IID)
	if err != nil {
		h.log.Debug().Err(err).Msg("MR review not found")
		return
	}

	now := time.Now()
	review.MergedAt = &now
	review.Status = models.MRStatusMerged

	if err := h.reviewRepo.UpdateMRReview(review); err != nil {
		h.log.Error().Err(err).Msg("Failed to update MR review")
		return
	}

	// Record Prometheus metrics for completed reviews
	// Get assignments for this review
	assignments, err := h.reviewRepo.GetAssignmentsByMRReviewID(review.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to get assignments for metrics")
		return
	}

	// Record completion for each reviewer
	for _, assignment := range assignments {
		if assignment.User.Username != "" {
			prommetrics.RecordReviewCompleted(review.Team, assignment.User.Username, assignment.Role)
		}
	}

	// Record histogram metrics
	h.recordHistogramMetrics(review, assignments)
}

// handleMRClosed updates the review status when MR is closed
func (h *Handler) handleMRClosed(_ context.Context, event MergeRequestEvent) {
	review, err := h.reviewRepo.GetMRReview(event.Project.ID, event.ObjectAttributes.IID)
	if err != nil {
		h.log.Debug().Err(err).Msg("MR review not found")
		return
	}

	now := time.Now()
	review.ClosedAt = &now
	review.Status = models.MRStatusClosed

	if err := h.reviewRepo.UpdateMRReview(review); err != nil {
		h.log.Error().Err(err).Msg("Failed to update MR review")
		return
	}

	// Record Prometheus metrics for abandoned reviews
	prommetrics.RecordReviewAbandoned(review.Team)
}

// handleMRApproved updates the review status when MR is approved
func (h *Handler) handleMRApproved(_ context.Context, event MergeRequestEvent) {
	review, err := h.reviewRepo.GetMRReview(event.Project.ID, event.ObjectAttributes.IID)
	if err != nil {
		h.log.Debug().Err(err).Msg("MR review not found for approval event")
		return
	}

	// Only update if not already set (first approval)
	if review.ApprovedAt == nil {
		now := time.Now()
		review.ApprovedAt = &now

		if err := h.reviewRepo.UpdateMRReview(review); err != nil {
			h.log.Error().Err(err).Msg("Failed to update MR review with approval time")
			return
		}

		h.log.Debug().
			Int("project_id", event.Project.ID).
			Int("mr_iid", event.ObjectAttributes.IID).
			Msg("MR approval recorded")
	}
}

// getOrCreateUser gets or creates a user from GitLab
func (h *Handler) getOrCreateUser(gitlabID int, _ string) (*models.User, error) {
	user, err := h.userRepo.GetByGitLabID(gitlabID)
	if err == nil {
		return user, nil
	}

	// User doesn't exist, fetch from GitLab and create
	glUser, err := h.gitlabClient.GetUser(gitlabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user from GitLab: %w", err)
	}

	user = &models.User{
		GitLabID: gitlabID,
		Username: glUser.Username,
		Email:    glUser.Email,
	}

	if err := h.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// recordHistogramMetrics calculates and records histogram metrics for Prometheus
func (h *Handler) recordHistogramMetrics(review *models.MRReview, assignments []models.ReviewerAssignment) {
	// Record Time to First Review (TTFR)
	if review.FirstReviewAt != nil && review.RouletteTriggeredAt != nil {
		ttfrSeconds := review.FirstReviewAt.Sub(*review.RouletteTriggeredAt).Seconds()
		if ttfrSeconds >= 0 {
			prommetrics.ObserveTTFR(review.Team, ttfrSeconds)
		}
	}

	// Record Time to Approval
	if review.ApprovedAt != nil && review.RouletteTriggeredAt != nil {
		approvalSeconds := review.ApprovedAt.Sub(*review.RouletteTriggeredAt).Seconds()
		if approvalSeconds >= 0 {
			prommetrics.ObserveTimeToApproval(review.Team, approvalSeconds)
		}
	}

	// Record comment metrics for each assignment
	for _, assignment := range assignments {
		if assignment.CommentCount > 0 {
			prommetrics.ObserveCommentCount(review.Team, float64(assignment.CommentCount))
		}
		if assignment.CommentLength > 0 {
			prommetrics.ObserveCommentLength(review.Team, float64(assignment.CommentLength))
		}
	}
}
