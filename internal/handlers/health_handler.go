package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

type HealthChecker interface {
	Check(ctx context.Context) error
}

type HealthHandler struct {
	readiness HealthChecker
}

func NewHealthHandler(readiness HealthChecker) *HealthHandler {
	return &HealthHandler{readiness: readiness}
}

func (h *HealthHandler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	if err := h.readiness.Check(c.UserContext()); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "unavailable",
		})
	}

	return c.JSON(fiber.Map{"status": "ready"})
}
