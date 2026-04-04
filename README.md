# About the Project

go-mcbots is a lightweight Minecraft bot framework written in Go. It is designed for developers who need a high-performance, low-overhead solution by stripping down a full-scale protocol library to its core essentials for bot automation.

# Features
 - Protocol 1.21.11 Support: Optimized for the latest Minecraft versions (including 1.21.1) with updated packet structures and state handling.
 - Minimalist Architecture: A clean codebase focused purely on bot logic, significantly reducing resource consumption compared to all-purpose libraries.
 - Efficient Concurrency: Leverages Go's goroutines to manage multiple bot instances simultaneously with high stability.
 - Modular Design: Easily extendable through dedicated packages for bot actions, game handling, and protocol management.

# Upcoming Features
I am actively planning to expand the framework's capabilities:
- Advanced Pathfinding: Implementing sophisticated navigation and path-finding algorithms for complex world traversal.
- Enhanced Bot Intelligence: Integrating smarter decision-making logic and autonomous behaviors to make bots more interactive.
- Extended World Interaction: Granular APIs for interacting with blocks, containers, and entities.

 # Installation
 ```
 go get github.com/Deware-PK/go-mcbots
 ```

# Quick Start
A simple example to connect a bot to a server:
```
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

```

# Credits & Acknowledgements
This project is forked and refactored from the following excellent open-source projects:

- Tnze/go-mc - The original Minecraft protocol implementation in Go.
- mj41/go-mc - Improved version with specific protocol stability.

Special thanks to the original authors for their contributions to the Go-Minecraft ecosystem.

# License
This project is licensed under the MIT License. See the LICENSE file for details.