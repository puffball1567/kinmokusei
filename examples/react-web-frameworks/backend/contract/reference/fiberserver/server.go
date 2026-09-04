//go:build kinmokusei_demo_contract

package fiberserver

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"example.com/kinmokusei/react-web-frameworks-backend/contract/reference"
	"github.com/gofiber/fiber/v3"
)

func NewApp() *fiber.App {
	store := reference.NewStore()
	app := fiber.New()

	app.Get("/api/health", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"status": "ok", "language": "Kinmokusei", "framework": "Fiber"})
	})
	app.Get("/api/todos", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"items": store.List()})
	})
	app.Post("/api/todos", func(ctx fiber.Ctx) error {
		var input struct {
			Title string `json:"title"`
		}
		if err := ctx.Bind().Body(&input); err != nil {
			return respondError(ctx, http.StatusBadRequest, "request body must be JSON with a title")
		}
		title := strings.TrimSpace(input.Title)
		if title == "" {
			return respondError(ctx, http.StatusBadRequest, "title is required")
		}
		if utf8.RuneCountInString(title) > 80 {
			return respondError(ctx, http.StatusBadRequest, "title must be 80 characters or fewer")
		}
		return ctx.Status(http.StatusCreated).JSON(fiber.Map{"item": store.Create(title)})
	})
	app.Patch("/api/todos/:id/toggle", func(ctx fiber.Ctx) error {
		id, valid := parseID(ctx.Params("id"))
		if !valid {
			return respondError(ctx, http.StatusBadRequest, "todo id must be a positive integer")
		}
		todo, found := store.Toggle(id)
		if !found {
			return respondError(ctx, http.StatusNotFound, "todo not found")
		}
		return ctx.JSON(fiber.Map{"item": todo})
	})
	app.Delete("/api/todos/:id", func(ctx fiber.Ctx) error {
		id, valid := parseID(ctx.Params("id"))
		if !valid {
			return respondError(ctx, http.StatusBadRequest, "todo id must be a positive integer")
		}
		if !store.Remove(id) {
			return respondError(ctx, http.StatusNotFound, "todo not found")
		}
		return ctx.SendStatus(http.StatusNoContent)
	})
	return app
}

func respondError(ctx fiber.Ctx, status int, message string) error {
	return ctx.Status(status).JSON(fiber.Map{"error": message})
}

func parseID(text string) (int, bool) {
	value, err := strconv.Atoi(text)
	return value, err == nil && value > 0
}
