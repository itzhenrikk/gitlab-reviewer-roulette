// Package health provides health check endpoints for monitoring the application status.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/cache"
	"github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// Handler handles health check endpoints
type Handler struct {
	db    *repository.DB
	cache *cache.Cache
	log   *logger.Logger
}

// NewHandler creates a new health check handler
func NewHandler(db *repository.DB, cacheClient *cache.Cache, log *logger.Logger) *Handler {
	return &Handler{
		db:    db,
		cache: cacheClient,
		log:   log,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks"`
	Version string            `json:"version"`
	Uptime  string            `json:"uptime"`
}

var startTime = time.Now()

// HandleHealth performs a health check
func (h *Handler) HandleHealth(c *gin.Context) {
	checks := make(map[string]string)
	overallStatus := "ok"

	// Check database
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.db.Health(); err != nil {
		checks["database"] = "error: " + err.Error()
		overallStatus = "degraded"
		h.log.Warn().Err(err).Msg("Database health check failed")
	} else {
		checks["database"] = "connected"
	}

	// Check Redis
	if err := h.cache.Health(ctx); err != nil {
		checks["redis"] = "error: " + err.Error()
		overallStatus = "degraded"
		h.log.Warn().Err(err).Msg("Redis health check failed")
	} else {
		checks["redis"] = "connected"
	}

	// Calculate uptime
	uptime := time.Since(startTime)

	response := HealthResponse{
		Status:  overallStatus,
		Checks:  checks,
		Version: "1.0.0", // TODO: Get from build info
		Uptime:  uptime.String(),
	}

	statusCode := http.StatusOK
	if overallStatus != "ok" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// HandleReadiness checks if the service is ready to accept requests
func (h *Handler) HandleReadiness(c *gin.Context) {
	// Check critical dependencies
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.db.Health(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": "database not ready",
		})
		return
	}

	if err := h.cache.Health(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": "cache not ready",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ready": true,
	})
}

// HandleLiveness checks if the service is alive
func (h *Handler) HandleLiveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"alive": true,
	})
}
