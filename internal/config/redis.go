package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultRedisAddress      = "localhost:6379"
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
	defaultRedisPoolSize     = 10
)

type Redis struct {
	Address      string
	Username     string
	Password     string
	Database     int
	TLSEnabled   bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

func LoadRedis() (Redis, error) {
	database, err := intFromEnvironment("REDIS_DB", 0)
	if err != nil {
		return Redis{}, err
	}
	tlsEnabled, err := boolFromEnvironment("REDIS_TLS_ENABLED", false)
	if err != nil {
		return Redis{}, err
	}
	dialTimeout, err := durationFromEnvironment("REDIS_DIAL_TIMEOUT", defaultRedisDialTimeout)
	if err != nil {
		return Redis{}, err
	}
	readTimeout, err := durationFromEnvironment("REDIS_READ_TIMEOUT", defaultRedisReadTimeout)
	if err != nil {
		return Redis{}, err
	}
	writeTimeout, err := durationFromEnvironment("REDIS_WRITE_TIMEOUT", defaultRedisWriteTimeout)
	if err != nil {
		return Redis{}, err
	}
	poolSize, err := intFromEnvironment("REDIS_POOL_SIZE", defaultRedisPoolSize)
	if err != nil {
		return Redis{}, err
	}
	if database < 0 {
		return Redis{}, fmt.Errorf("REDIS_DB must be non-negative")
	}
	if poolSize < 1 {
		return Redis{}, fmt.Errorf("REDIS_POOL_SIZE must be positive")
	}

	address := os.Getenv("REDIS_ADDR")
	if address == "" {
		address = defaultRedisAddress
	}

	return Redis{
		Address:      address,
		Username:     os.Getenv("REDIS_USERNAME"),
		Password:     os.Getenv("REDIS_PASSWORD"),
		Database:     database,
		TLSEnabled:   tlsEnabled,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolSize:     poolSize,
	}, nil
}

func intFromEnvironment(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}

	return value, nil
}

func boolFromEnvironment(name string, fallback bool) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}

	return value, nil
}

func durationFromEnvironment(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}

	return value, nil
}
