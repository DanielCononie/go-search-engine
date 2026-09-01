package redis

import (
	"crypto/tls"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

func NewClient(redisConfig config.Redis) *goredis.Client {
	options := &goredis.Options{
		Addr:         redisConfig.Address,
		Username:     redisConfig.Username,
		Password:     redisConfig.Password,
		DB:           redisConfig.Database,
		DialTimeout:  redisConfig.DialTimeout,
		ReadTimeout:  redisConfig.ReadTimeout,
		WriteTimeout: redisConfig.WriteTimeout,
		PoolSize:     redisConfig.PoolSize,
		Protocol:     2,
	}
	if redisConfig.TLSEnabled {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return goredis.NewClient(options)
}
