package main

import (
	"log"

	"github.com/DanielCononie/go-search-engine.git/go-search-engine/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	app.Get("/search", handlers.Search)

	log.Fatal(app.Listen(":3000"))
}
