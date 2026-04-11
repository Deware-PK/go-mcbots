package service

import (
	"fmt"
	"log/slog"

	"github.com/deware-pk/go-mcbots/internal/model"
	"github.com/deware-pk/go-mcbots/pkg/bot"
	"github.com/deware-pk/go-mcbots/pkg/pool"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
)

type BotService interface {
	LaunchBot(req *model.LaunchRequest) error
	ListBots() []string
	RemoveBot(id string) error
	Chat(id string, req *model.ChatRequest) error
	GoTo(id string, req *model.GoToRequest) error
	Stop(id string) error
	GetStatus(id string) (map[string]interface{}, error)
}

type botService struct {
	pool *pool.BotPool
}

func NewBotService(p *pool.BotPool) BotService {
	return &botService{
		pool: p,
	}
}

func (s *botService) LaunchBot(req *model.LaunchRequest) error {
	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		return fmt.Errorf("failed to resolve protocol: %w", err)
	}

	setup := func(b *bot.Bot) {
		b.Events.OnDisconnect = func(reason string) {
			slog.Info("Bot disconnected", "id", req.ID, "reason", reason)
		}
	}

	if err := s.pool.Launch(req.ID, req.Name, ver, req.Addr, setup); err != nil {
		return fmt.Errorf("failed to launch bot: %w", err)
	}

	return nil
}

func (s *botService) ListBots() []string {
	return s.pool.GetAllIDs()
}

func (s *botService) RemoveBot(id string) error {
	if _, ok := s.pool.Get(id); !ok {
		return fmt.Errorf("bot not found")
	}

	s.pool.Remove(id)
	return nil
}

func (s *botService) Chat(id string, req *model.ChatRequest) error {
	return s.pool.Execute(id, func(b *bot.Bot) error {
		return b.Chat(req.Message)
	})
}

func (s *botService) GoTo(id string, req *model.GoToRequest) error {
	return s.pool.Execute(id, func(b *bot.Bot) error {
		return b.GoTo(req.X, req.Y, req.Z, req.Sprint)
	})
}

func (s *botService) Stop(id string) error {
	return s.pool.Execute(id, func(b *bot.Bot) error {
		b.StopPathfinding()
		return nil
	})
}

func (s *botService) GetStatus(id string) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := s.pool.Execute(id, func(b *bot.Bot) error {
		x, y, z := b.GetPosition()
		health, food := b.GetHealth()
		current, total := b.GetPathProgress()

		result = map[string]interface{}{
			"id":    id,
			"alive": b.IsAlive(),
			"position": map[string]interface{}{
				"x": x, "y": y, "z": z,
			},
			"health":     health,
			"food":       food,
			"navigating": b.IsNavigating(),
			"path_progress": map[string]interface{}{
				"current": current,
				"total":   total,
			},
		}
		return nil
	})

	return result, err
}
