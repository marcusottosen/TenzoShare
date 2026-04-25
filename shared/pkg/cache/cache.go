// Package cache provides a thin Redis wrapper for all TenzoShare services.
// It exposes the operations needed for rate limiting, session storage, and
// short-lived key/value storage without exposing the raw Redis client.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// ErrNotFound is returned when a key does not exist in the cache.
var ErrNotFound = errors.New("cache: key not found")

// Client wraps a Redis client.
type Client struct {
	rdb *redis.Client
}

// New creates a new Client from the given RedisConfig and verifies connectivity.
func New(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

// Set stores value under key with the given TTL. Pass 0 for no expiry.
func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get returns the value stored under key. Returns ErrNotFound if the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return val, err
}

// Del deletes one or more keys. Missing keys are not an error.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Incr atomically increments key by 1 and returns the new value.
// If the key does not exist it is initialised to 0 before incrementing.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// Expire sets a TTL on an existing key. Returns false if the key does not exist.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.rdb.Expire(ctx, key, ttl).Result()
}

// SetNX sets key to value with TTL only if the key does not already exist.
// Returns true if the key was set, false if it already existed.
func (c *Client) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// TTL returns the remaining time-to-live for key. Returns -1 if no TTL is set,
// -2 if the key does not exist.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// Close closes the underlying Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// RevokeToken adds a JTI to the access-token revocation blacklist.
// ttl should match the remaining lifetime of the access token so entries
// self-expire from Redis automatically.
func (c *Client) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	return c.rdb.Set(ctx, "revoked:jti:"+jti, "1", ttl).Err()
}

// IsTokenRevoked reports whether a JTI is on the revocation blacklist.
func (c *Client) IsTokenRevoked(ctx context.Context, jti string) bool {
	err := c.rdb.Get(ctx, "revoked:jti:"+jti).Err()
	return err == nil // key present → revoked; Nil error means key exists
}
