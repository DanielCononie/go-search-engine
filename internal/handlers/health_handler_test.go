package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type stubHealthChecker struct {
	err error
}

func (c stubHealthChecker) Check(_ context.Context) error {
	return c.err
}

func TestHealthHandlerReady(t *testing.T) {
	app := fiber.New()
	handler := NewHealthHandler(stubHealthChecker{})
	app.Get("/health/ready", handler.Ready)

	response, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
}

func TestHealthHandlerUnavailable(t *testing.T) {
	app := fiber.New()
	handler := NewHealthHandler(stubHealthChecker{err: errors.New("Redis unavailable")})
	app.Get("/health/ready", handler.Ready)

	response, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusServiceUnavailable)
	}
}
