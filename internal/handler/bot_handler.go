package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/deware-pk/go-mcbots/internal/model"
	"github.com/deware-pk/go-mcbots/internal/service"
)

type BotHandler struct {
	svc service.BotService
}

func NewBotHandler(svc service.BotService) *BotHandler {
	return &BotHandler{
		svc: svc,
	}
}

func (h *BotHandler) LaunchBot(c *gin.Context) {
	var req model.LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.LaunchBot(&req); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "launched", "id": req.ID})
}

func (h *BotHandler) ListBots(c *gin.Context) {
	ids := h.svc.ListBots()
	c.JSON(http.StatusOK, gin.H{"bots": ids, "count": len(ids)})
}

func (h *BotHandler) RemoveBot(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.RemoveBot(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed", "id": id})
}

func (h *BotHandler) Chat(c *gin.Context) {
	id := c.Param("id")

	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Chat(id, &req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent", "id": id, "message": req.Message})
}

func (h *BotHandler) GoTo(c *gin.Context) {
	id := c.Param("id")

	var req model.GoToRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.GoTo(id, &req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "navigating",
		"id":     id,
		"target": gin.H{"x": req.X, "y": req.Y, "z": req.Z},
	})
}

func (h *BotHandler) Stop(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Stop(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped", "id": id})
}

func (h *BotHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
