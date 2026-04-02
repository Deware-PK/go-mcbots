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
	"log"
	"github.com/Deware-PK/go-mcbots/pkg/bot"
)

func main() {
	// Initialize a new bot instance
	b := bot.New("MyBotName")
	
	// Join a server
	err := b.JoinServer("localhost:25565")
	if err != nil {
		log.Fatal(err)
	}
	
	// Handle game events
	b.HandleGame()
}
```

# Credits & Acknowledgements
This project is forked and refactored from the following excellent open-source projects:

- Tnze/go-mc - The original Minecraft protocol implementation in Go.
- mj41/go-mc - Improved version with specific protocol stability.

Special thanks to the original authors for their contributions to the Go-Minecraft ecosystem.

# License
This project is licensed under the MIT License. See the LICENSE file for details.