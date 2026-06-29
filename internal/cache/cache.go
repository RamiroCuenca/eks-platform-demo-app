// Package cache wraps a go-redis client to ElastiCache. The connection uses TLS
// (in-transit encryption) and an AUTH token, matching the replication group config.
package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/RamiroCuenca/eks-platform-demo-app/internal/config"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client   *redis.Client
	queueKey string
}

// New builds a Redis client. Like the DB pool it connects lazily.
func New(cfg config.RedisConfig) (*Cache, error) {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
	}
	if cfg.TLS {
		host, _, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			return nil, fmt.Errorf("parse redis addr %q: %w", cfg.Addr, err)
		}
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		}
	}
	return &Cache{client: redis.NewClient(opts), queueKey: cfg.QueueKey}, nil
}

// Ping verifies connectivity for readiness checks.
func (c *Cache) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.Ping(ctx).Err()
}

// RoundTrip writes a value and reads it straight back — proves TLS+AUTH connectivity.
func (c *Cache) RoundTrip(ctx context.Context, key, val string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.client.Set(ctx, key, val, time.Minute).Err(); err != nil {
		return "", err
	}
	return c.client.Get(ctx, key).Result()
}

// Enqueue pushes a job onto the work list and returns the new list length — the
// signal KEDA's Redis scaler reads to decide worker replica count.
func (c *Cache) Enqueue(ctx context.Context, payload string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.LPush(ctx, c.queueKey, payload).Result()
}

// Dequeue blocks until a job is available or the timeout elapses.
func (c *Cache) Dequeue(ctx context.Context, timeout time.Duration) (string, error) {
	res, err := c.client.BRPop(ctx, timeout, c.queueKey).Result()
	if err != nil {
		return "", err
	}
	// BRPOP returns [key, value].
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected BRPOP result length %d", len(res))
	}
	return res[1], nil
}

// QueueLen reports the current backlog.
func (c *Cache) QueueLen(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.LLen(ctx, c.queueKey).Result()
}

func (c *Cache) Close() error { return c.client.Close() }
