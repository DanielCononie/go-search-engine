package config

import (
	"testing"
	"time"
)

func TestLoadRedisDefaults(t *testing.T) {
	for _, name := range []string{
		"REDIS_ADDR",
		"REDIS_USERNAME",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_TLS_ENABLED",
		"REDIS_DIAL_TIMEOUT",
		"REDIS_READ_TIMEOUT",
		"REDIS_WRITE_TIMEOUT",
		"REDIS_POOL_SIZE",
	} {
		t.Setenv(name, "")
	}

	redisConfig, err := LoadRedis()
	if err != nil {
		t.Fatal(err)
	}

	if redisConfig.Address != defaultRedisAddress {
		t.Fatalf("address = %q, want %q", redisConfig.Address, defaultRedisAddress)
	}
	if redisConfig.Database != 0 || redisConfig.TLSEnabled {
		t.Fatalf("database = %d, TLS enabled = %t", redisConfig.Database, redisConfig.TLSEnabled)
	}
	if redisConfig.PoolSize != defaultRedisPoolSize {
		t.Fatalf("pool size = %d, want %d", redisConfig.PoolSize, defaultRedisPoolSize)
	}
}

func TestLoadRedisEnvironment(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis.example.com:6380")
	t.Setenv("REDIS_USERNAME", "search-api")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_TLS_ENABLED", "true")
	t.Setenv("REDIS_DIAL_TIMEOUT", "4s")
	t.Setenv("REDIS_READ_TIMEOUT", "2s")
	t.Setenv("REDIS_WRITE_TIMEOUT", "2500ms")
	t.Setenv("REDIS_POOL_SIZE", "20")

	redisConfig, err := LoadRedis()
	if err != nil {
		t.Fatal(err)
	}

	if redisConfig.Address != "redis.example.com:6380" {
		t.Fatalf("address = %q", redisConfig.Address)
	}
	if redisConfig.Username != "search-api" || redisConfig.Password != "secret" {
		t.Fatal("credentials were not loaded")
	}
	if redisConfig.Database != 2 || !redisConfig.TLSEnabled {
		t.Fatalf("database = %d, TLS enabled = %t", redisConfig.Database, redisConfig.TLSEnabled)
	}
	if redisConfig.DialTimeout != 4*time.Second ||
		redisConfig.ReadTimeout != 2*time.Second ||
		redisConfig.WriteTimeout != 2500*time.Millisecond ||
		redisConfig.PoolSize != 20 {
		t.Fatal("timeouts or pool size were not loaded")
	}
}

func TestLoadRedisRejectsInvalidValues(t *testing.T) {
	t.Setenv("REDIS_POOL_SIZE", "0")

	if _, err := LoadRedis(); err == nil {
		t.Fatal("expected invalid pool size error")
	}
}
