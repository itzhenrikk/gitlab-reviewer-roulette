// Package main provides the initialization tool for GitLab Reviewer Roulette.
// It syncs users and merge requests from GitLab to the database.
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/gitlab"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	groupID    = flag.Int("group", 0, "GitLab group ID to sync users from (optional)")
	groupPath  = flag.String("group-path", "", "GitLab group path to sync users from (e.g., 'test-org')")
	projectID  = flag.Int("project", 0, "GitLab project ID to sync MRs from (optional)")
	syncUsers  = flag.Bool("users", true, "Sync users from GitLab")
	syncMRs    = flag.Bool("mrs", true, "Sync open merge requests")
	dryRun     = flag.Bool("dry-run", false, "Dry run mode (don't write to database)")
	maxMRs     = flag.Int("max-mrs", 100, "Maximum number of MRs to sync per project")
)

func main() {
	flag.Parse()

	// Initialize logger
	logger.Init("info", "console", "stdout")
	log := logger.Get()

	log.Info().Msg("🚀 Starting GitLab Reviewer Roulette Initialization")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Connect to database
	db, err := repository.NewDB(&cfg.Database.Postgres, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize GitLab client
	gitlabClient, err := gitlab.NewClient(&cfg.GitLab, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize GitLab client")
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	reviewRepo := repository.NewReviewRepository(db)

	ctx := context.Background()

	// Resolve group ID if group path is provided
	resolvedGroupID := *groupID
	if *groupPath != "" {
		log.Info().Str("path", *groupPath).Msg("Looking up group by path...")
		group, err := gitlabClient.GetGroupByPath(*groupPath)
		if err != nil {
			log.Fatal().Err(err).Str("path", *groupPath).Msg("Failed to lookup group by path")
		}
		resolvedGroupID = group.ID
		log.Info().Int("group_id", resolvedGroupID).Str("name", group.Name).Msg("Found group")
	}

	// Sync users from GitLab
	if *syncUsers {
		log.Info().Msg("📥 Syncing users from GitLab...")

		// Always sync from config.yaml first to get team information
		log.Info().Msg("Syncing users from config.yaml teams...")
		if err := syncUsersFromConfig(cfg, gitlabClient, userRepo, *dryRun); err != nil {
			log.Warn().Err(err).Msg("Failed to sync users from config (continuing with group sync)")
		}

		// Then supplement with additional users from group or project if specified
		if resolvedGroupID > 0 {
			log.Info().Msg("Supplementing with additional users from group...")
			if err := syncUsersFromGroup(ctx, gitlabClient, userRepo, resolvedGroupID, *dryRun); err != nil {
				log.Error().Err(err).Msg("Failed to sync users from group")
			}
		} else if *projectID > 0 {
			log.Info().Msg("Supplementing with additional users from project...")
			if err := syncUsersFromProject(ctx, gitlabClient, userRepo, *projectID, *dryRun); err != nil {
				log.Error().Err(err).Msg("Failed to sync users from project")
			}
		}
	}

	// Sync open merge requests
	if *syncMRs {
		log.Info().Msg("📥 Syncing open merge requests...")

		switch {
		case *projectID > 0:
			// Sync from specific project
			if err := syncMRsFromProject(ctx, gitlabClient, userRepo, reviewRepo, *projectID, *maxMRs, *dryRun); err != nil {
				log.Error().Err(err).Msg("Failed to sync MRs from project")
			}
		case resolvedGroupID > 0:
			// Sync from all projects in group
			if err := syncMRsFromGroup(ctx, gitlabClient, userRepo, reviewRepo, resolvedGroupID, *maxMRs, *dryRun); err != nil {
				log.Error().Err(err).Msg("Failed to sync MRs from group")
			}
		default:
			log.Warn().Msg("No group or project specified for MR sync. Skipping.")
		}
	}

	log.Info().Msg("✅ Initialization complete!")
}

// syncUsersFromGroup syncs all members of a GitLab group
func syncUsersFromGroup(ctx context.Context, gitlabClient *gitlab.Client, userRepo *repository.UserRepository, groupID int, dryRun bool) error {
	log := logger.Get()

	members, err := gitlabClient.GetGroupMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group members: %w", err)
	}

	log.Info().Int("count", len(members)).Msg("Found group members")

	synced := 0
	for _, member := range members {
		// Check if user already exists
		existingUser, err := userRepo.GetByGitLabID(member.ID)
		if err == nil && existingUser != nil {
			log.Debug().
				Int("gitlab_id", member.ID).
				Str("username", member.Username).
				Msg("User already exists, skipping")
			continue
		}

		if dryRun {
			log.Info().
				Int("gitlab_id", member.ID).
				Str("username", member.Username).
				Str("email", member.Email).
				Msg("[DRY RUN] Would create user")
			synced++
			continue
		}

		// Create new user
		user := &models.User{
			GitLabID:  member.ID,
			Username:  member.Username,
			Email:     member.Email,
			Role:      detectRole(member), // Try to detect role from user info
			Team:      "",                 // Will be updated when assigned to team
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := userRepo.CreateOrUpdate(user); err != nil {
			log.Error().Err(err).
				Str("username", member.Username).
				Msg("Failed to create/update user")
			continue
		}

		log.Info().
			Int("gitlab_id", member.ID).
			Str("username", member.Username).
			Msg("✓ Created/updated user")
		synced++
	}

	log.Info().Int("synced", synced).Int("total", len(members)).Msg("User sync complete")
	return nil
}

// syncUsersFromProject syncs all members of a GitLab project
func syncUsersFromProject(ctx context.Context, gitlabClient *gitlab.Client, userRepo *repository.UserRepository, projectID int, dryRun bool) error {
	log := logger.Get()

	members, err := gitlabClient.GetProjectMembers(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project members: %w", err)
	}

	log.Info().Int("count", len(members)).Msg("Found project members")

	synced := 0
	for _, member := range members {
		existingUser, err := userRepo.GetByGitLabID(member.ID)
		if err == nil && existingUser != nil {
			log.Debug().
				Int("gitlab_id", member.ID).
				Str("username", member.Username).
				Msg("User already exists, skipping")
			continue
		}

		if dryRun {
			log.Info().
				Int("gitlab_id", member.ID).
				Str("username", member.Username).
				Msg("[DRY RUN] Would create user")
			synced++
			continue
		}

		user := &models.User{
			GitLabID:  member.ID,
			Username:  member.Username,
			Email:     member.Email,
			Role:      detectRole(member),
			Team:      "",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := userRepo.CreateOrUpdate(user); err != nil {
			log.Error().Err(err).
				Str("username", member.Username).
				Msg("Failed to create/update user")
			continue
		}

		log.Info().
			Int("gitlab_id", member.ID).
			Str("username", member.Username).
			Msg("✓ Created/updated user")
		synced++
	}

	log.Info().Int("synced", synced).Int("total", len(members)).Msg("User sync complete")
	return nil
}

// syncUsersFromConfig syncs users from config.yaml teams (existing behavior)
func syncUsersFromConfig(cfg *config.Config, gitlabClient *gitlab.Client, userRepo *repository.UserRepository, dryRun bool) error {
	log := logger.Get()

	synced := 0
	for _, team := range cfg.Teams {
		for _, member := range team.Members {
			existingUser, err := userRepo.GetByUsername(member.Username)
			if err == nil && existingUser != nil {
				log.Debug().
					Str("username", member.Username).
					Str("team", team.Name).
					Msg("User already exists, skipping")
				continue
			}

			if dryRun {
				log.Info().
					Str("username", member.Username).
					Str("team", team.Name).
					Str("role", member.Role).
					Msg("[DRY RUN] Would create user")
				synced++
				continue
			}

			// Fetch GitLab user to get actual ID and email
			gitlabUser, err := gitlabClient.GetUserByUsername(member.Username)
			if err != nil {
				log.Warn().Err(err).
					Str("username", member.Username).
					Msg("Could not fetch GitLab user, skipping")
				// Skip this user instead of creating with ID 0
				continue
			}

			user := &models.User{
				GitLabID:  gitlabUser.ID,
				Username:  member.Username,
				Email:     gitlabUser.Email,
				Role:      member.Role,
				Team:      team.Name,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := userRepo.Create(user); err != nil {
				log.Error().Err(err).
					Str("username", member.Username).
					Msg("Failed to create user")
				continue
			}

			log.Info().
				Str("username", member.Username).
				Str("team", team.Name).
				Msg("✓ Created user")
			synced++
		}
	}

	log.Info().Int("synced", synced).Msg("Config sync complete")
	return nil
}

// syncMRsFromProject syncs open merge requests from a specific project
func syncMRsFromProject(ctx context.Context, gitlabClient *gitlab.Client, userRepo *repository.UserRepository, reviewRepo *repository.ReviewRepository, projectID, maxMRs int, dryRun bool) error {
	log := logger.Get()

	mrs, err := gitlabClient.GetOpenMergeRequests(projectID, maxMRs)
	if err != nil {
		return fmt.Errorf("failed to get open MRs: %w", err)
	}

	log.Info().Int("count", len(mrs)).Int("project_id", projectID).Msg("Found open merge requests")

	synced := 0
	for _, mr := range mrs {
		// Check if MR already tracked
		existing, err := reviewRepo.GetByProjectAndMR(projectID, mr.IID)
		if err == nil && existing != nil {
			log.Debug().
				Int("project_id", projectID).
				Int("mr_iid", mr.IID).
				Msg("MR already tracked, skipping")
			continue
		}

		if dryRun {
			log.Info().
				Int("project_id", projectID).
				Int("mr_iid", mr.IID).
				Str("title", mr.Title).
				Str("author", mr.Author.Username).
				Msg("[DRY RUN] Would track MR")
			synced++
			continue
		}

		// Get or create author
		author, err := getOrCreateUser(userRepo, gitlabClient, mr.Author.ID, mr.Author.Username, "")
		if err != nil {
			log.Error().Err(err).
				Str("username", mr.Author.Username).
				Msg("Failed to get/create author")
			continue
		}

		// Create MR review record
		review := &models.MRReview{
			GitLabMRIID:     mr.IID,
			GitLabProjectID: projectID,
			MRURL:           mr.WebURL,
			MRTitle:         mr.Title,
			MRAuthorID:      &author.ID,
			Team:            detectTeamFromLabels(mr.Labels),
			Status:          "open",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		if err := reviewRepo.CreateMRReview(review); err != nil {
			log.Error().Err(err).
				Int("mr_iid", mr.IID).
				Msg("Failed to create MR review")
			continue
		}

		// Sync existing reviewers (assignees/reviewers from GitLab)
		if err := syncMRReviewers(gitlabClient, userRepo, reviewRepo, review, mr); err != nil {
			log.Warn().Err(err).
				Int("mr_iid", mr.IID).
				Msg("Failed to sync MR reviewers")
		}

		log.Info().
			Int("project_id", projectID).
			Int("mr_iid", mr.IID).
			Str("title", mr.Title).
			Msg("✓ Tracked MR")
		synced++
	}

	log.Info().Int("synced", synced).Int("total", len(mrs)).Msg("MR sync complete")
	return nil
}

// syncMRsFromGroup syncs open MRs from all projects in a group
func syncMRsFromGroup(ctx context.Context, gitlabClient *gitlab.Client, userRepo *repository.UserRepository, reviewRepo *repository.ReviewRepository, groupID, maxMRs int, dryRun bool) error {
	log := logger.Get()

	projects, err := gitlabClient.GetGroupProjects(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group projects: %w", err)
	}

	log.Info().Int("count", len(projects)).Int("group_id", groupID).Msg("Found projects in group")

	for _, project := range projects {
		log.Info().
			Int("project_id", project.ID).
			Str("project_name", project.Name).
			Msg("Syncing MRs from project...")

		if err := syncMRsFromProject(ctx, gitlabClient, userRepo, reviewRepo, project.ID, maxMRs, dryRun); err != nil {
			log.Error().Err(err).
				Int("project_id", project.ID).
				Msg("Failed to sync MRs from project")
			// Continue with other projects
		}
	}

	return nil
}

// Helper functions

func detectRole(member interface{}) string {
	// Try to detect role from GitLab user info
	// Could check access level, job title, etc.
	// For now, default to "dev"
	return "dev"
}

func detectTeamFromLabels(labels []string) string {
	for _, label := range labels {
		if len(label) > 6 && label[:6] == "name::" {
			return label[6:]
		}
	}
	return ""
}

func getOrCreateUser(userRepo *repository.UserRepository, gitlabClient *gitlab.Client, gitlabID int, username, email string) (*models.User, error) {
	// Try to find by GitLab ID
	user, err := userRepo.GetByGitLabID(gitlabID)
	if err == nil && user != nil {
		return user, nil
	}

	// Try to find by username
	user, err = userRepo.GetByUsername(username)
	if err == nil && user != nil {
		// Update GitLab ID if it was 0
		if user.GitLabID == 0 {
			user.GitLabID = gitlabID
			if err := userRepo.Update(user); err != nil {
				return nil, err
			}
		}
		return user, nil
	}

	// Create new user
	user = &models.User{
		GitLabID:  gitlabID,
		Username:  username,
		Email:     email,
		Role:      "dev", // Default role
		Team:      "",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func syncMRReviewers(gitlabClient *gitlab.Client, userRepo *repository.UserRepository, reviewRepo *repository.ReviewRepository, review *models.MRReview, mr interface{}) error {
	// This would sync existing reviewers/assignees from the MR
	// Implementation depends on GitLab API response structure
	// For now, just a placeholder
	return nil
}
