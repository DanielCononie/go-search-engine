package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type HealthChecker struct {
	client *goredis.Client
}

func NewHealthChecker(client *goredis.Client) *HealthChecker {
	return &HealthChecker{client: client}
}

func (h *HealthChecker) Check(ctx context.Context) error {
	return h.client.Ping(ctx).Err()
}

type ReadinessChecker struct {
	health  *HealthChecker
	indexes *IndexManager
}

func NewReadinessChecker(health *HealthChecker, indexes *IndexManager) *ReadinessChecker {
	return &ReadinessChecker{
		health:  health,
		indexes: indexes,
	}
}

func (c *ReadinessChecker) Check(ctx context.Context) error {
	if err := c.health.Check(ctx); err != nil {
		return err
	}

	return c.indexes.Check(ctx)
}
