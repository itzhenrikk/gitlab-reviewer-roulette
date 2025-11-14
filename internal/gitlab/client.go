package gitlab

import (
	"encoding/base64"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// Client wraps the GitLab API client.
type Client struct {
	client *gitlab.Client
	log    *logger.Logger
	config *config.GitLabConfig
}

// NewClient creates a new GitLab client.
func NewClient(cfg *config.GitLabConfig, log *logger.Logger) (*Client, error) {
	client, err := gitlab.NewClient(cfg.Token, gitlab.WithBaseURL(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	log.Info().
		Str("url", cfg.URL).
		Str("bot_username", cfg.BotUsername).
		Msg("GitLab client initialized")

	return &Client{
		client: client,
		log:    log,
		config: cfg,
	}, nil
}

// GetUser retrieves a user by ID.
func (c *Client) GetUser(userID int) (*gitlab.User, error) {
	user, _, err := c.client.Users.GetUser(userID, gitlab.GetUsersOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user %d: %w", userID, err)
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username.
func (c *Client) GetUserByUsername(username string) (*gitlab.User, error) {
	users, _, err := c.client.Users.ListUsers(&gitlab.ListUsersOptions{
		Username: gitlab.Ptr(username),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", username, err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user %s not found", username)
	}
	return users[0], nil
}

// GetMergeRequest retrieves a merge request.
func (c *Client) GetMergeRequest(projectID, mrIID int) (*gitlab.MergeRequest, error) {
	mr, _, err := c.client.MergeRequests.GetMergeRequest(projectID, mrIID, &gitlab.GetMergeRequestsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get MR %d in project %d: %w", mrIID, projectID, err)
	}
	return mr, nil
}

// GetMergeRequestChanges retrieves the file changes in a merge request.
func (c *Client) GetMergeRequestChanges(projectID, mrIID int) ([]*gitlab.MergeRequestDiff, error) {
	// Use ListMergeRequestDiffs instead of GetMergeRequestChanges (deprecated)
	diffs, _, err := c.client.MergeRequests.ListMergeRequestDiffs(projectID, mrIID, &gitlab.ListMergeRequestDiffsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get MR changes %d in project %d: %w", mrIID, projectID, err)
	}
	return diffs, nil
}

// PostComment posts a comment on a merge request and returns the note ID.
func (c *Client) PostComment(projectID, mrIID int, comment string) (int, error) {
	note, _, err := c.client.Notes.CreateMergeRequestNote(projectID, mrIID, &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.Ptr(comment),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to post comment on MR %d in project %d: %w", mrIID, projectID, err)
	}

	c.log.Debug().
		Int("project_id", projectID).
		Int("mr_iid", mrIID).
		Int("note_id", note.ID).
		Msg("Posted comment to MR")

	return note.ID, nil
}

// UpdateComment updates an existing comment on a merge request.
func (c *Client) UpdateComment(projectID, mrIID, noteID int, comment string) error {
	_, _, err := c.client.Notes.UpdateMergeRequestNote(projectID, mrIID, noteID, &gitlab.UpdateMergeRequestNoteOptions{
		Body: gitlab.Ptr(comment),
	})
	if err != nil {
		return fmt.Errorf("failed to update comment %d on MR %d in project %d: %w", noteID, mrIID, projectID, err)
	}

	c.log.Debug().
		Int("project_id", projectID).
		Int("mr_iid", mrIID).
		Int("note_id", noteID).
		Msg("Updated comment on MR")

	return nil
}

// GetCodeowners retrieves the CODEOWNERS file content.
func (c *Client) GetCodeowners(projectID int, ref string) (string, error) {
	// Try different CODEOWNERS file locations
	paths := []string{
		"CODEOWNERS",
		".gitlab/CODEOWNERS",
		"docs/CODEOWNERS",
	}

	for _, path := range paths {
		file, _, err := c.client.RepositoryFiles.GetFile(projectID, path, &gitlab.GetFileOptions{
			Ref: gitlab.Ptr(ref),
		})
		if err == nil {
			// File content is base64 encoded in the Content field
			if file.Content != "" {
				// Try to decode from base64
				decoded, err := base64.StdEncoding.DecodeString(file.Content)
				if err != nil {
					return "", fmt.Errorf("failed to decode CODEOWNERS: %w", err)
				}
				return string(decoded), nil
			}
			return "", fmt.Errorf("CODEOWNERS file is empty")
		}
	}

	return "", fmt.Errorf("CODEOWNERS file not found")
}

// ParseCodeowners parses a CODEOWNERS file content and returns a map of patterns to owners.
func ParseCodeowners(content string) map[string][]string {
	owners := make(map[string][]string)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		pattern := parts[0]
		usernames := make([]string, 0)

		for _, part := range parts[1:] {
			if strings.HasPrefix(part, "@") {
				// Remove @ prefix
				username := strings.TrimPrefix(part, "@")
				usernames = append(usernames, username)
			}
		}

		if len(usernames) > 0 {
			owners[pattern] = usernames
		}
	}

	return owners
}

// GetUserStatus retrieves the GitLab user status. Note: User status API might not be available in all GitLab versions.
func (c *Client) GetUserStatus(userID int) (*UserStatus, error) {
	// Try to get user details which may include status
	user, _, err := c.client.Users.GetUser(userID, gitlab.GetUsersOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user %d: %w", userID, err)
	}

	// If user has a status, return it
	// Note: Status field might not be available in older GitLab versions
	if user.State == "blocked" || user.State == "banned" {
		return &UserStatus{
			Availability: "busy",
			Message:      user.State,
		}, nil
	}

	// Check if user has custom status (this is a newer GitLab feature)
	// For now, return nil to indicate no special status
	return nil, nil
}

// UserStatus represents a simplified user status.
type UserStatus struct {
	Availability string // "busy" or empty
	Message      string // status message
}

// GetProjectLabels retrieves all labels for a project.
func (c *Client) GetProjectLabels(projectID int) ([]*gitlab.Label, error) {
	labels, _, err := c.client.Labels.ListLabels(projectID, &gitlab.ListLabelsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get labels for project %d: %w", projectID, err)
	}
	return labels, nil
}

// GetMergeRequestNotes retrieves all notes (comments) for a merge request.
func (c *Client) GetMergeRequestNotes(projectID, mrIID int) ([]*gitlab.Note, error) {
	var allNotes []*gitlab.Note
	page := 1

	for {
		notes, resp, err := c.client.Notes.ListMergeRequestNotes(projectID, mrIID, &gitlab.ListMergeRequestNotesOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get notes for MR %d in project %d: %w", mrIID, projectID, err)
		}

		allNotes = append(allNotes, notes...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allNotes, nil
}

// GetMergeRequestApprovals retrieves approval information for a merge request.
func (c *Client) GetMergeRequestApprovals(projectID, mrIID int) (*gitlab.MergeRequestApprovals, error) {
	approvals, _, err := c.client.MergeRequestApprovals.GetConfiguration(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approvals for MR %d in project %d: %w", mrIID, projectID, err)
	}

	c.log.Debug().
		Int("project_id", projectID).
		Int("mr_iid", mrIID).
		Int("approvals_count", len(approvals.ApprovedBy)).
		Msg("Retrieved MR approvals")

	return approvals, nil
}

// ListOpenMergeRequests lists all open merge requests in a project.
func (c *Client) ListOpenMergeRequests(projectID int) ([]*gitlab.BasicMergeRequest, error) {
	var allMRs []*gitlab.BasicMergeRequest
	page := 1

	for {
		mrs, resp, err := c.client.MergeRequests.ListProjectMergeRequests(projectID, &gitlab.ListProjectMergeRequestsOptions{
			State: gitlab.Ptr("opened"),
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list open MRs for project %d: %w", projectID, err)
		}

		allMRs = append(allMRs, mrs...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allMRs, nil
}

// IsUserAvailable checks if a user is available based on their status.
func IsUserAvailable(status *UserStatus, oooKeywords []string) bool {
	if status == nil {
		return true // No status means available
	}

	// Check if user is busy
	if status.Availability == "busy" {
		return false
	}

	// Check if status message contains OOO keywords
	if status.Message != "" {
		messageLower := strings.ToLower(status.Message)
		for _, keyword := range oooKeywords {
			if strings.Contains(messageLower, strings.ToLower(keyword)) {
				return false
			}
		}
	}

	return true
}

// GetGroupByPath retrieves a GitLab group by its path.
func (c *Client) GetGroupByPath(path string) (*gitlab.Group, error) {
	group, _, err := c.client.Groups.GetGroup(path, &gitlab.GetGroupOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get group by path %s: %w", path, err)
	}
	return group, nil
}

// GetGroupMembers retrieves all members of a GitLab group.
func (c *Client) GetGroupMembers(groupID int) ([]*gitlab.GroupMember, error) {
	var allMembers []*gitlab.GroupMember
	page := 1

	for {
		members, resp, err := c.client.Groups.ListGroupMembers(groupID, &gitlab.ListGroupMembersOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get group members for group %d: %w", groupID, err)
		}

		allMembers = append(allMembers, members...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allMembers, nil
}

// GetProjectMembers retrieves all members of a GitLab project.
func (c *Client) GetProjectMembers(projectID int) ([]*gitlab.ProjectMember, error) {
	var allMembers []*gitlab.ProjectMember
	page := 1

	for {
		members, resp, err := c.client.ProjectMembers.ListProjectMembers(projectID, &gitlab.ListProjectMembersOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get project members for project %d: %w", projectID, err)
		}

		allMembers = append(allMembers, members...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allMembers, nil
}

// GetOpenMergeRequests retrieves open merge requests from a project.
func (c *Client) GetOpenMergeRequests(projectID, maxMRs int) ([]*gitlab.BasicMergeRequest, error) {
	perPage := 100
	if maxMRs < perPage {
		perPage = maxMRs
	}

	var allMRs []*gitlab.BasicMergeRequest
	page := 1

	for {
		mrs, resp, err := c.client.MergeRequests.ListProjectMergeRequests(projectID, &gitlab.ListProjectMergeRequestsOptions{
			State: gitlab.Ptr("opened"),
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: perPage,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get open MRs for project %d: %w", projectID, err)
		}

		allMRs = append(allMRs, mrs...)

		// Stop if we've reached the maximum
		if len(allMRs) >= maxMRs || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	// Trim to maxMRs if we exceeded
	if len(allMRs) > maxMRs {
		allMRs = allMRs[:maxMRs]
	}

	return allMRs, nil
}

// GetGroupProjects retrieves all projects in a GitLab group.
func (c *Client) GetGroupProjects(groupID int) ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	page := 1

	for {
		projects, resp, err := c.client.Groups.ListGroupProjects(groupID, &gitlab.ListGroupProjectsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get group projects for group %d: %w", groupID, err)
		}

		allProjects = append(allProjects, projects...)

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allProjects, nil
}
