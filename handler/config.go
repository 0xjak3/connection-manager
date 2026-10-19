package handler

import "github.com/gofiber/fiber/v3"

// FiberConfig is the single source of the Fiber app config. router.New uses
// it, and handler tests build fiber.New(FiberConfig()).
func FiberConfig() fiber.Config {
	return fiber.Config{
		ErrorHandler:    ErrorHandler,
		StructValidator: NewStructValidator(),
	}
}
