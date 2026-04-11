package router

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/deware-pk/go-mcbots/internal/handler"
)

// SetupRouter creates the gin router, adds middlewares, and registers routes
func SetupRouter(botHandler *handler.BotHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Slog Logger middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		slog.Info("HTTP Request",
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"latency", latency,
			"client_ip", c.ClientIP(),
		)
	})

	r.Use(gin.Recovery())

	// Register Routes
	r.POST("/bots", botHandler.LaunchBot)
	r.GET("/bots", botHandler.ListBots)
	r.DELETE("/bots/:id", botHandler.RemoveBot)
	r.POST("/bots/:id/chat", botHandler.Chat)
	r.POST("/bots/:id/goto", botHandler.GoTo)
	r.POST("/bots/:id/stop", botHandler.Stop)
	r.GET("/bots/:id/status", botHandler.GetStatus)

	return r
}
