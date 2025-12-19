package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Client   *redis.Client
}

// ConnectRedis connects to Redis and returns a client
func (r *RedisConfig) ConnectRedis() error {
	r.Client = redis.NewClient(&redis.Options{
		Addr:     r.Addr,
		Password: r.Password,
		DB:       r.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test the connection
	_, err := r.Client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("✅ Connected to Redis")
	return nil
}

// Disconnect closes the Redis connection
func (r *RedisConfig) Disconnect() error {
	if r.Client == nil {
		return nil
	}

	return r.Client.Close()
}

// IsConnected checks if Redis is connected
func (r *RedisConfig) IsConnected() bool {
	if r.Client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := r.Client.Ping(ctx).Result()
	return err == nil
}

// GetCacheKey generates a standardized cache key
func (r *RedisConfig) GetCacheKey(prefix, identifier string) string {
	return fmt.Sprintf("%s:%s", prefix, identifier)
}