package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deware-pk/go-mcbots/internal/config"
	"github.com/deware-pk/go-mcbots/internal/handler"
	"github.com/deware-pk/go-mcbots/internal/router"
	"github.com/deware-pk/go-mcbots/internal/service"
	"github.com/deware-pk/go-mcbots/pkg/bot"
	"github.com/deware-pk/go-mcbots/pkg/pool"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
)

func main() {
	// 1. Load config and logger
	cfg := config.LoadConfig()
	config.SetupLogger(cfg)

	slog.Info("Starting go-mcbots API...")

	// 2. Setup pool
	p := pool.New()

	// 3. Setup Layers
	botService := service.NewBotService(p)
	botHandler := handler.NewBotHandler(botService)
	r := router.SetupRouter(botHandler)

	// 4. Start Server
	go func() {
		slog.Info("Listening on", "port", cfg.Port)
		if err := r.Run(":" + cfg.Port); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// 5. Launch a demo bot via the pool (Retained from previous main.go)
	launchDemoBot(p)

	// 6. Graceful shutdown on interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down...")
	p.Shutdown()
	slog.Info("All bots disconnected. Bye!")
}

// launchDemoBot launches the demo bot
func launchDemoBot(p *pool.BotPool) {
	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		slog.Error("Failed to resolve protocol for demo bot", "error", err)
		return
	}

	setup := func(b *bot.Bot) {
		b.Events.OnSpawn = func() {
			x, y, z := b.GetPosition()
			slog.Info("Spawned", "x", x, "y", y, "z", z)

			b.Chat("Hello from Go bot!")
			b.Command("gamemode creative")

			// Walk forward for 3 seconds
			b.SetControlState("forward", true)
			b.SetControlState("sprint", true)
			time.AfterFunc(3*time.Second, func() {
				b.ClearControlStates()
				slog.Info("Stopped walking")
				x, y, z := b.GetPosition()
				slog.Info("Now at", "x", x, "y", y, "z", z)
			})
		}

		b.Events.OnChat = func(sender, message string) {
			slog.Info("Chat", "sender", sender, "message", message)
		}

		b.Events.OnSystemMessage = func(message string) {
			slog.Info("System", "message", message)
		}

		b.Events.OnHealthUpdate = func(health, food float32) {
			slog.Info("Health Update", "health", health, "food", food)
		}

		b.Events.OnDeath = func() {
			slog.Info("Bot died! Respawning...")
			b.Respawn()
		}

		b.Events.OnDisconnect = func(reason string) {
			slog.Info("Disconnected", "reason", reason)
		}
	}

	if err := p.Launch("demo-bot", "GoBot", ver, "localhost:25565", setup); err != nil {
		slog.Error("Failed to launch demo bot", "error", err)
	}
}
