// Package main is the entry point for the GitLab Reviewer Roulette server.
// It initializes the application, sets up database connections, and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/api/dashboard"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/api/health"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/api/webhook"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/cache"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/gitlab"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/i18n"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/mattermost"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/models"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/badges"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/leaderboard"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/metrics"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/roulette"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/service/scheduler"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	log := logger.Get()

	log.Info().
		Str("environment", cfg.Server.Environment).
		Int("port", cfg.Server.Port).
		Msg("Starting GitLab Reviewer Roulette Bot")

	// Initialize database
	db, err := repository.NewDB(&cfg.Database.Postgres, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Initialize Redis cache
	redisCache, err := cache.NewCache(&cfg.Database.Redis, log)
	if err != nil {
		db.Close()
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisCache.Close()

	// Initialize GitLab client
	gitlabClient, err := gitlab.NewClient(&cfg.GitLab, log)
	if err != nil {
		redisCache.Close()
		db.Close()
		log.Fatal().Err(err).Msg("Failed to create GitLab client")
	}

	// Initialize Mattermost client
	mattermostClient := mattermost.NewClient(&cfg.Mattermost, log)

	// Initialize translator for i18n
	translator, err := i18n.New(cfg.Server.Language)
	if err != nil {
		redisCache.Close()
		db.Close()
		log.Fatal().Err(err).Msg("Failed to initialize translator")
	}
	log.Info().
		Str("language", translator.Lang()).
		Msg("Translator initialized")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	oooRepo := repository.NewOOORepository(db)
	reviewRepo := repository.NewReviewRepository(db)
	metricsRepo := repository.NewMetricsRepository(db)
	badgeRepo := repository.NewBadgeRepository(db)

	// Sync users from config to database
	if err := syncUsersFromConfig(cfg, userRepo, log); err != nil {
		log.Warn().Err(err).Msg("Failed to sync users from config")
	}

	// Initialize services
	rouletteService := roulette.NewService(
		cfg,
		gitlabClient,
		userRepo,
		oooRepo,
		reviewRepo,
		redisCache,
		log,
	)

	metricsService := metrics.NewService(metricsRepo)

	badgeService := badges.NewService(
		badgeRepo,
		metricsRepo,
		reviewRepo,
		userRepo,
		log,
	)

	leaderboardService := leaderboard.NewService(
		metricsRepo,
		badgeRepo,
		userRepo,
		log,
	)

	schedulerService := scheduler.NewService(
		cfg,
		reviewRepo,
		badgeService,
		mattermostClient,
		log,
	)

	// Initialize handlers
	webhookHandler := webhook.NewHandler(
		cfg,
		gitlabClient,
		mattermostClient,
		rouletteService,
		metricsService,
		userRepo,
		reviewRepo,
		translator,
		log,
	)

	healthHandler := health.NewHandler(db, redisCache, log)

	dashboardHandler := dashboard.NewHandler(badgeService, leaderboardService, log)

	// Setup Gin router
	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Health endpoints
	router.GET("/health", healthHandler.HandleHealth)
	router.GET("/readiness", healthHandler.HandleReadiness)
	router.GET("/liveness", healthHandler.HandleLiveness)

	// Webhook endpoint
	router.POST("/webhook/gitlab", webhookHandler.HandleGitLabWebhook)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Dashboard endpoints (read-only, no authentication required)
		// These endpoints are safe for public access and provide statistics/leaderboards
		v1.GET("/leaderboard", dashboardHandler.GetGlobalLeaderboard)
		v1.GET("/leaderboard/:team", dashboardHandler.GetTeamLeaderboard)
		v1.GET("/users/:id/stats", dashboardHandler.GetUserStats)
		v1.GET("/users/:id/badges", dashboardHandler.GetUserBadges)
		v1.GET("/badges", dashboardHandler.GetBadgeCatalog)
		v1.GET("/badges/:id", dashboardHandler.GetBadgeByID)
		v1.GET("/badges/:id/holders", dashboardHandler.GetBadgeHolders)

		// Admin endpoints (Phase 6 - Not yet implemented)
		// TODO: Add OIDC authentication middleware before enabling these endpoints
		// - POST   /api/v1/ooo                 - Create OOO status
		// - DELETE /api/v1/ooo/:id             - Delete OOO status
		// - POST   /api/v1/badges/:id/award    - Manually award badge
		// - DELETE /api/v1/users/:id/badges/:badge_id - Revoke badge
		// - PUT    /api/v1/users/:id           - Update user info

		// Health check endpoint
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})
	}

	// Start scheduler if enabled
	if cfg.Scheduler.Enabled {
		if err := schedulerService.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start scheduler")
		}
		defer schedulerService.Stop()
	}

	// Start Prometheus metrics server
	if cfg.Metrics.Prometheus.Enabled {
		go startMetricsServer(cfg.Metrics.Prometheus.Port, cfg.Metrics.Prometheus.Path, log)
	}

	// Setup HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second, // Prevents Slowloris attacks
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Str("address", addr).
			Msg("Starting HTTP server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			redisCache.Close()
			db.Close()
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited")
}

// startMetricsServer starts the Prometheus metrics server.
func startMetricsServer(port int, path string, log *logger.Logger) {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	addr := fmt.Sprintf(":%d", port)
	log.Info().
		Str("address", addr).
		Str("path", path).
		Msg("Starting Prometheus metrics server")

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Error().Err(err).Msg("Metrics server failed")
	}
}

// syncUsersFromConfig syncs users from config file to database
func syncUsersFromConfig(cfg *config.Config, userRepo *repository.UserRepository, log *logger.Logger) error {
	log.Info().Msg("Syncing users from config to database")

	for _, team := range cfg.Teams {
		for _, member := range team.Members {
			// Check if user exists by username
			_, err := userRepo.GetByUsername(member.Username)
			if err == nil {
				// User exists, skip
				continue
			}

			// User doesn't exist, create placeholder (will be updated when they interact with GitLab)
			user := &models.User{
				GitLabID: 0, // Will be updated later
				Username: member.Username,
				Role:     member.Role,
				Team:     team.Name,
			}

			if err := userRepo.Create(user); err != nil {
				log.Warn().
					Str("username", member.Username).
					Err(err).
					Msg("Failed to create user")
				continue
			}

			log.Debug().
				Str("username", member.Username).
				Str("team", team.Name).
				Str("role", member.Role).
				Msg("Created user from config")
		}
	}

	log.Info().Msg("User sync completed")
	return nil
}
