package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
	"github.com/aimd54/gitlab-reviewer-roulette/pkg/logger"
)

// setupTestCache creates a cache instance with miniredis (in-memory Redis)
func setupTestCache(t *testing.T) (*Cache, *miniredis.Miniredis, func()) {
	t.Helper()

	// Start miniredis (in-memory Redis server)
	mr, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")

	// Initialize logger (level, format, output)
	logger.Init("error", "json", "stderr")
	log := logger.Get()

	// Create cache config pointing to miniredis
	cfg := &config.RedisConfig{
		Host:     mr.Host(),
		Port:     mr.Server().Addr().Port,
		Password: "",
		DB:       0,
		PoolSize: 10,
	}

	// Create cache instance
	cache, err := NewCache(cfg, log)
	require.NoError(t, err, "Failed to create cache")

	// Cleanup function
	cleanup := func() {
		_ = cache.Close()
		mr.Close()
	}

	return cache, mr, cleanup
}

func TestNewCache(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		cache, _, cleanup := setupTestCache(t)
		defer cleanup()

		assert.NotNil(t, cache)
		assert.NotNil(t, cache.client)
	})

	t.Run("failed connection", func(t *testing.T) {
		logger.Init("error", "json", "stderr")
		log := logger.Get()
		cfg := &config.RedisConfig{
			Host:     "invalid-host",
			Port:     9999,
			Password: "",
			DB:       0,
			PoolSize: 10,
		}

		cache, err := NewCache(cfg, log)
		assert.Error(t, err)
		assert.Nil(t, cache)
	})
}

func TestCache_Get(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("get existing key", func(t *testing.T) {
		// Set a value directly in miniredis
		_ = mr.Set("test-key", "test-value")

		val, err := cache.Get(ctx, "test-key")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", val)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		val, err := cache.Get(ctx, "non-existent")
		assert.NoError(t, err)
		assert.Equal(t, "", val) // Returns empty string for non-existent keys
	})
}

func TestCache_Set(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("set string value", func(t *testing.T) {
		err := cache.Set(ctx, "key1", "value1", 5*time.Minute)
		assert.NoError(t, err)

		// Verify in miniredis
		val, err := mr.Get("key1")
		assert.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("set with expiration", func(t *testing.T) {
		err := cache.Set(ctx, "key2", "value2", 10*time.Second)
		assert.NoError(t, err)

		// Check TTL
		ttl := mr.TTL("key2")
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, 10*time.Second)
	})

	t.Run("set integer value", func(t *testing.T) {
		err := cache.Set(ctx, "key3", 123, 5*time.Minute)
		assert.NoError(t, err)

		val, err := cache.Get(ctx, "key3")
		assert.NoError(t, err)
		assert.Equal(t, "123", val)
	})
}

func TestCache_Del(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("delete existing key", func(t *testing.T) {
		_ = mr.Set("key-to-delete", "value")

		err := cache.Del(ctx, "key-to-delete")
		assert.NoError(t, err)

		exists := mr.Exists("key-to-delete")
		assert.False(t, exists)
	})

	t.Run("delete multiple keys", func(t *testing.T) {
		_ = mr.Set("key1", "val1")
		_ = mr.Set("key2", "val2")
		_ = mr.Set("key3", "val3")

		err := cache.Del(ctx, "key1", "key2", "key3")
		assert.NoError(t, err)

		assert.False(t, mr.Exists("key1"))
		assert.False(t, mr.Exists("key2"))
		assert.False(t, mr.Exists("key3"))
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		err := cache.Del(ctx, "non-existent")
		assert.NoError(t, err) // Should not error
	})
}

func TestCache_Exists(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("check existing key", func(t *testing.T) {
		_ = mr.Set("existing-key", "value")

		count, err := cache.Exists(ctx, "existing-key")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("check non-existent key", func(t *testing.T) {
		count, err := cache.Exists(ctx, "non-existent")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("check multiple keys", func(t *testing.T) {
		_ = mr.Set("key1", "val1")
		_ = mr.Set("key2", "val2")

		count, err := cache.Exists(ctx, "key1", "key2", "key3")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count) // Only 2 exist
	})
}

func TestCache_Incr(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("increment non-existent key", func(t *testing.T) {
		val, err := cache.Incr(ctx, "counter1")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), val)
	})

	t.Run("increment existing key", func(t *testing.T) {
		_ = mr.Set("counter2", "5")

		val, err := cache.Incr(ctx, "counter2")
		assert.NoError(t, err)
		assert.Equal(t, int64(6), val)
	})

	t.Run("multiple increments", func(t *testing.T) {
		val1, _ := cache.Incr(ctx, "counter3")
		val2, _ := cache.Incr(ctx, "counter3")
		val3, _ := cache.Incr(ctx, "counter3")

		assert.Equal(t, int64(1), val1)
		assert.Equal(t, int64(2), val2)
		assert.Equal(t, int64(3), val3)
	})
}

func TestCache_Decr(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("decrement non-existent key", func(t *testing.T) {
		val, err := cache.Decr(ctx, "counter1")
		assert.NoError(t, err)
		assert.Equal(t, int64(-1), val)
	})

	t.Run("decrement existing key", func(t *testing.T) {
		_ = mr.Set("counter2", "10")

		val, err := cache.Decr(ctx, "counter2")
		assert.NoError(t, err)
		assert.Equal(t, int64(9), val)
	})
}

func TestCache_SAdd(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("add single member", func(t *testing.T) {
		err := cache.SAdd(ctx, "set1", "member1")
		assert.NoError(t, err)

		members, _ := mr.SMembers("set1")
		assert.Contains(t, members, "member1")
	})

	t.Run("add multiple members", func(t *testing.T) {
		err := cache.SAdd(ctx, "set2", "m1", "m2", "m3")
		assert.NoError(t, err)

		members, _ := mr.SMembers("set2")
		assert.Len(t, members, 3)
		assert.Contains(t, members, "m1")
		assert.Contains(t, members, "m2")
		assert.Contains(t, members, "m3")
	})

	t.Run("add duplicate member", func(t *testing.T) {
		_ = cache.SAdd(ctx, "set3", "member")
		err := cache.SAdd(ctx, "set3", "member") // Add again
		assert.NoError(t, err)

		members, _ := mr.SMembers("set3")
		assert.Len(t, members, 1) // Still only 1 member
	})
}

func TestCache_SRem(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("remove existing member", func(t *testing.T) {
		_, _ = mr.SetAdd("set1", "m1", "m2", "m3")

		err := cache.SRem(ctx, "set1", "m2")
		assert.NoError(t, err)

		members, _ := mr.SMembers("set1")
		assert.Len(t, members, 2)
		assert.NotContains(t, members, "m2")
	})

	t.Run("remove non-existent member", func(t *testing.T) {
		_, _ = mr.SetAdd("set2", "m1")

		err := cache.SRem(ctx, "set2", "m2")
		assert.NoError(t, err) // Should not error
	})
}

func TestCache_SMembers(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("get members from populated set", func(t *testing.T) {
		_, _ = mr.SetAdd("set1", "m1", "m2", "m3")

		members, err := cache.SMembers(ctx, "set1")
		assert.NoError(t, err)
		assert.Len(t, members, 3)
		assert.Contains(t, members, "m1")
		assert.Contains(t, members, "m2")
		assert.Contains(t, members, "m3")
	})

	t.Run("get members from empty set", func(t *testing.T) {
		members, err := cache.SMembers(ctx, "non-existent-set")
		assert.NoError(t, err)
		assert.Empty(t, members)
	})
}

func TestCache_SIsMember(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("check existing member", func(t *testing.T) {
		_, _ = mr.SetAdd("set1", "member1")

		exists, err := cache.SIsMember(ctx, "set1", "member1")
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("check non-existent member", func(t *testing.T) {
		_, _ = mr.SetAdd("set2", "member1")

		exists, err := cache.SIsMember(ctx, "set2", "member2")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestCache_SetNX(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("set non-existent key", func(t *testing.T) {
		ok, err := cache.SetNX(ctx, "lock1", "value1", 10*time.Second)
		assert.NoError(t, err)
		assert.True(t, ok)

		val, _ := mr.Get("lock1")
		assert.Equal(t, "value1", val)
	})

	t.Run("set existing key", func(t *testing.T) {
		_ = mr.Set("lock2", "existing")

		ok, err := cache.SetNX(ctx, "lock2", "new-value", 10*time.Second)
		assert.NoError(t, err)
		assert.False(t, ok) // Should fail

		val, _ := mr.Get("lock2")
		assert.Equal(t, "existing", val) // Value unchanged
	})

	t.Run("distributed lock pattern", func(t *testing.T) {
		// First lock acquisition
		ok1, _ := cache.SetNX(ctx, "resource-lock", "client1", 5*time.Second)
		assert.True(t, ok1)

		// Second attempt should fail
		ok2, _ := cache.SetNX(ctx, "resource-lock", "client2", 5*time.Second)
		assert.False(t, ok2)
	})
}

func TestCache_Expire(t *testing.T) {
	cache, mr, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("set expiration on existing key", func(t *testing.T) {
		_ = mr.Set("key1", "value")

		err := cache.Expire(ctx, "key1", 30*time.Second)
		assert.NoError(t, err)

		ttl := mr.TTL("key1")
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, 30*time.Second)
	})

	t.Run("set expiration on non-existent key", func(t *testing.T) {
		err := cache.Expire(ctx, "non-existent", 10*time.Second)
		assert.NoError(t, err) // Redis doesn't error for this
	})
}

func TestCache_Health(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("healthy connection", func(t *testing.T) {
		err := cache.Health(ctx)
		assert.NoError(t, err)
	})
}

func TestCache_Close(t *testing.T) {
	cache, mr, _ := setupTestCache(t)

	err := cache.Close()
	assert.NoError(t, err)

	mr.Close()
}

// Test cache key constants
func TestCacheKeyConstants(t *testing.T) {
	assert.Equal(t, "user:availability:%d", KeyUserAvailability)
	assert.Equal(t, "user:review_count:%d", KeyUserReviewCount)
	assert.Equal(t, "user:recent_reviews:%d", KeyUserRecentReviews)
	assert.Equal(t, "mr:pending", KeyPendingMRs)
	assert.Equal(t, "config:teams", KeyConfigTeams)
}

// Integration test: simulating real usage pattern
func TestCache_RealWorldUsage(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("user availability caching", func(t *testing.T) {
		cacheKey := "user:availability:42"

		// First check - cache miss
		val, _ := cache.Get(ctx, cacheKey)
		assert.Equal(t, "", val)

		// Store availability
		_ = cache.Set(ctx, cacheKey, "available", 5*time.Minute)

		// Second check - cache hit
		val, err := cache.Get(ctx, cacheKey)
		assert.NoError(t, err)
		assert.Equal(t, "available", val)
	})

	t.Run("review count caching", func(t *testing.T) {
		cacheKey := "user:review_count:42"

		// Cache review count
		_ = cache.Set(ctx, cacheKey, "5", 5*time.Minute)

		// Retrieve
		val, err := cache.Get(ctx, cacheKey)
		assert.NoError(t, err)
		assert.Equal(t, "5", val)
	})

	t.Run("pending MRs set", func(t *testing.T) {
		setKey := "mr:pending"

		// Add pending MRs
		_ = cache.SAdd(ctx, setKey, "123:1", "123:2", "456:1")

		// Check membership
		exists, _ := cache.SIsMember(ctx, setKey, "123:1")
		assert.True(t, exists)

		// Get all pending
		members, _ := cache.SMembers(ctx, setKey)
		assert.Len(t, members, 3)

		// Remove one
		_ = cache.SRem(ctx, setKey, "123:1")

		members, _ = cache.SMembers(ctx, setKey)
		assert.Len(t, members, 2)
	})
}
