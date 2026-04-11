package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deware-pk/go-mcbots/pkg/api"
	"github.com/deware-pk/go-mcbots/pkg/bot"
	"github.com/deware-pk/go-mcbots/pkg/pool"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
)

func main() {
	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		log.Fatal(err)
	}

	p := pool.New()

	// Start HTTP API on :8080
	srv := api.NewServer(p)
	go func() {
		log.Println("[API] Listening on :8080")
		if err := srv.Router().Run(":8080"); err != nil {
			log.Fatalf("[API] Server error: %v", err)
		}
	}()

	// Launch a demo bot via the pool
	setup := func(b *bot.Bot) {
		b.Events.OnSpawn = func() {
			x, y, z := b.GetPosition()
			log.Printf("Spawned at X=%.2f Y=%.2f Z=%.2f", x, y, z)

			b.Chat("Hello from Go bot!")
			b.Command("gamemode creative")

			// Walk forward for 3 seconds
			b.SetControlState("forward", true)
			b.SetControlState("sprint", true)
			time.AfterFunc(3*time.Second, func() {
				b.ClearControlStates()
				log.Println("Stopped walking")
				x, y, z := b.GetPosition()
				log.Printf("Now at X=%.2f Y=%.2f Z=%.2f", x, y, z)
			})
		}

		b.Events.OnChat = func(sender, message string) {
			log.Printf("[Chat] <%s> %s", sender, message)
		}

		b.Events.OnSystemMessage = func(message string) {
			log.Printf("[System] %s", message)
		}

		b.Events.OnHealthUpdate = func(health, food float32) {
			log.Printf("Health: %.1f, Food: %.1f", health, food)
		}

		b.Events.OnDeath = func() {
			log.Println("Bot died! Respawning...")
			b.Respawn()
		}

		b.Events.OnDisconnect = func(reason string) {
			log.Printf("Disconnected: %s", reason)
		}
	}

	if err := p.Launch("demo-bot", "GoBot", ver, "localhost:25565", setup); err != nil {
		log.Printf("[Main] Failed to launch demo bot: %v", err)
	}

	// Graceful shutdown on interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[Main] Shutting down...")
	p.Shutdown()
	log.Println("[Main] All bots disconnected. Bye!")
}
