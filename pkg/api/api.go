package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/deware-pk/go-mcbots/pkg/bot"
	"github.com/deware-pk/go-mcbots/pkg/pool"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
)

type Server struct {
	pool   *pool.BotPool
	router *gin.Engine
}

func NewServer(p *pool.BotPool) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	s := &Server{
		pool:   p,
		router: r,
	}

	r.POST("/bots", s.handleLaunchBot)
	r.GET("/bots", s.handleListBots)
	r.DELETE("/bots/:id", s.handleRemoveBot)
	r.POST("/bots/:id/chat", s.handleChat)
	r.POST("/bots/:id/goto", s.handleGoTo)
	r.POST("/bots/:id/stop", s.handleStop)
	r.GET("/bots/:id/status", s.handleStatus)

	return s
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

type launchRequest struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
	Addr string `json:"addr" binding:"required"`
}

func (s *Server) handleLaunchBot(c *gin.Context) {
	var req launchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	setup := func(b *bot.Bot) {
		b.Events.OnDisconnect = func(reason string) {
			log.Printf("[API] Bot %q disconnected: %s", req.ID, reason)
		}
	}

	if err := s.pool.Launch(req.ID, req.Name, ver, req.Addr, setup); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "launched", "id": req.ID})
}

func (s *Server) handleListBots(c *gin.Context) {
	ids := s.pool.GetAllIDs()
	c.JSON(http.StatusOK, gin.H{"bots": ids, "count": len(ids)})
}

func (s *Server) handleRemoveBot(c *gin.Context) {
	id := c.Param("id")

	if _, ok := s.pool.Get(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	s.pool.Remove(id)
	c.JSON(http.StatusOK, gin.H{"status": "removed", "id": id})
}

type chatRequest struct {
	Message string `json:"message" binding:"required"`
}

func (s *Server) handleChat(c *gin.Context) {
	id := c.Param("id")

	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.pool.Execute(id, func(b *bot.Bot) error {
		return b.Chat(req.Message)
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent", "id": id, "message": req.Message})
}

type goToRequest struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Sprint bool    `json:"sprint"`
}

func (s *Server) handleGoTo(c *gin.Context) {
	id := c.Param("id")

	var req goToRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.pool.Execute(id, func(b *bot.Bot) error {
		return b.GoTo(req.X, req.Y, req.Z, req.Sprint)
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "navigating",
		"id":     id,
		"target": gin.H{"x": req.X, "y": req.Y, "z": req.Z},
	})
}

func (s *Server) handleStop(c *gin.Context) {
	id := c.Param("id")

	err := s.pool.Execute(id, func(b *bot.Bot) error {
		b.StopPathfinding()
		return nil
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped", "id": id})
}

func (s *Server) handleStatus(c *gin.Context) {
	id := c.Param("id")

	var result gin.H
	err := s.pool.Execute(id, func(b *bot.Bot) error {
		x, y, z := b.GetPosition()
		health, food := b.GetHealth()
		current, total := b.GetPathProgress()

		result = gin.H{
			"id":    id,
			"alive": b.IsAlive(),
			"position": gin.H{
				"x": x, "y": y, "z": z,
			},
			"health":     health,
			"food":       food,
			"navigating": b.IsNavigating(),
			"path_progress": gin.H{
				"current": current,
				"total":   total,
			},
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
