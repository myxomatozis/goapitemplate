package cache

import (
	"context"
	"fmt"
	"time"

	"goapitemplate/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/bradfitz/gomemcache/memcache"
)

type Client interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type RedisClient struct {
	client *redis.Client
}

type MemcacheClient struct {
	client *memcache.Client
}

func New(cfg config.CacheConfig) (Client, error) {
	switch cfg.Type {
	case "redis":
		return newRedisClient(cfg)
	case "memcache":
		return newMemcacheClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Type)
	}
}

func newRedisClient(cfg config.CacheConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

func newMemcacheClient(cfg config.CacheConfig) (*MemcacheClient, error) {
	client := memcache.New(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	client.Timeout = 5 * time.Second

	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to Memcache: %w", err)
	}

	return &MemcacheClient{client: client}, nil
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

func (r *RedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (m *MemcacheClient) Get(ctx context.Context, key string) (string, error) {
	item, err := m.client.Get(key)
	if err == memcache.ErrCacheMiss {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(item.Value), nil
}

func (m *MemcacheClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	item := &memcache.Item{
		Key:        key,
		Value:      []byte(value),
		Expiration: int32(ttl.Seconds()),
	}
	return m.client.Set(item)
}

func (m *MemcacheClient) Delete(ctx context.Context, key string) error {
	return m.client.Delete(key)
}

func (m *MemcacheClient) Close() error {
	return nil
}