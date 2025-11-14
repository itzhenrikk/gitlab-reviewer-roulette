// Package cache provides Redis client wrapper for caching operations.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// Cache wraps Redis client.
type Cache struct {
	client *redis.Client
	log    *logger.Logger
}

// NewCache creates a new Redis cache client.
func NewCache(cfg *config.RedisConfig, log *logger.Logger) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Int("db", cfg.DB).
		Msg("Connected to Redis")

	return &Cache{
		client: client,
		log:    log,
	}, nil
}

// Get retrieves a value from cache.
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key doesn't exist
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Set stores a value in cache with expiration.
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	err := c.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// Del deletes a key from cache.
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}
	return nil
}

// Exists checks if a key exists.
func (c *Cache) Exists(ctx context.Context, keys ...string) (int64, error) {
	count, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to check key existence: %w", err)
	}
	return count, nil
}

// Incr increments a key's value.
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return val, nil
}

// Decr decrements a key's value.
func (c *Cache) Decr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return val, nil
}

// SAdd adds members to a set.
func (c *Cache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	err := c.client.SAdd(ctx, key, members...).Err()
	if err != nil {
		return fmt.Errorf("failed to add to set %s: %w", key, err)
	}
	return nil
}

// SRem removes members from a set.
func (c *Cache) SRem(ctx context.Context, key string, members ...interface{}) error {
	err := c.client.SRem(ctx, key, members...).Err()
	if err != nil {
		return fmt.Errorf("failed to remove from set %s: %w", key, err)
	}
	return nil
}

// SMembers returns all members of a set.
func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := c.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get set members %s: %w", key, err)
	}
	return members, nil
}

// SIsMember checks if a member exists in a set.
func (c *Cache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	exists, err := c.client.SIsMember(ctx, key, member).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check set membership %s: %w", key, err)
	}
	return exists, nil
}

// SetNX sets a key only if it doesn't exist (for distributed locking).
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	ok, err := c.client.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return ok, nil
}

// Expire sets an expiration on a key.
func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	err := c.client.Expire(ctx, key, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on key %s: %w", key, err)
	}
	return nil
}

// Close closes the Redis connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Health checks if Redis is healthy.
func (c *Cache) Health(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Cache key constants.
const (
	KeyUserAvailability  = "user:availability:%d"   // user:availability:{gitlab_id}
	KeyUserReviewCount   = "user:review_count:%d"   // user:review_count:{gitlab_id}
	KeyUserRecentReviews = "user:recent_reviews:%d" // user:recent_reviews:{gitlab_id}
	KeyPendingMRs        = "mr:pending"             // set of "project_id:mr_iid"
	KeyConfigTeams       = "config:teams"           // JSON of team configuration
)
