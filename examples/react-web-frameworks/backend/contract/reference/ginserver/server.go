//go:build kinmokusei_demo_contract

package ginserver

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"example.com/kinmokusei/react-web-frameworks-backend/contract/reference"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	store := reference.NewStore()
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/api/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok", "language": "Kinmokusei", "framework": "Gin"})
	})
	router.GET("/api/todos", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"items": store.List()})
	})
	router.POST("/api/todos", func(ctx *gin.Context) {
		var input struct {
			Title string `json:"title"`
		}
		if err := ctx.ShouldBindJSON(&input); err != nil {
			respondError(ctx, http.StatusBadRequest, "request body must be JSON with a title")
			return
		}
		title := strings.TrimSpace(input.Title)
		if title == "" {
			respondError(ctx, http.StatusBadRequest, "title is required")
			return
		}
		if utf8.RuneCountInString(title) > 80 {
			respondError(ctx, http.StatusBadRequest, "title must be 80 characters or fewer")
			return
		}
		ctx.JSON(http.StatusCreated, gin.H{"item": store.Create(title)})
	})
	router.PATCH("/api/todos/:id/toggle", func(ctx *gin.Context) {
		id, valid := parseID(ctx.Param("id"))
		if !valid {
			respondError(ctx, http.StatusBadRequest, "todo id must be a positive integer")
			return
		}
		todo, found := store.Toggle(id)
		if !found {
			respondError(ctx, http.StatusNotFound, "todo not found")
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"item": todo})
	})
	router.DELETE("/api/todos/:id", func(ctx *gin.Context) {
		id, valid := parseID(ctx.Param("id"))
		if !valid {
			respondError(ctx, http.StatusBadRequest, "todo id must be a positive integer")
			return
		}
		if !store.Remove(id) {
			respondError(ctx, http.StatusNotFound, "todo not found")
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	return router
}

func respondError(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, gin.H{"error": message})
}

func parseID(text string) (int, bool) {
	value, err := strconv.Atoi(text)
	return value, err == nil && value > 0
}
