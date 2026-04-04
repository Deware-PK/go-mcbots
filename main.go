package main

import (
	"go-mcbots/pkg/bot"
	"go-mcbots/pkg/protocol"
	"log"
	"time"
)

func main() {
	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		log.Fatal(err)
	}

	b := bot.New("GoBot", ver)

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

	if err := b.Connect("localhost:25565"); err != nil {
		log.Fatal(err)
	}
}
