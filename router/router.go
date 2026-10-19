package router

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"connection_manager/handler"
)

// Handlers holds every HTTP handler. Built in cmd/api/main.go.
type Handlers struct {
	User *handler.UserHandler
}

// New creates the Fiber app and registers every route.
func New(h Handlers) *fiber.App {
	app := fiber.New(handler.FiberConfig())
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/healthz", healthz)

	v1 := app.Group("/api/v1")
	v1.Post("/users", h.User.Create)
	v1.Get("/users", h.User.List)
	v1.Get("/users/:id", h.User.Get)
	v1.Patch("/users/:id", h.User.Update)
	v1.Delete("/users/:id", h.User.Delete)

	return app
}

func healthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
