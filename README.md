# go-mcbots

A lightweight Minecraft bot framework and REST API server written in Go. Run and control multiple Minecraft bots simultaneously over HTTP — launch bots, send chat, navigate to coordinates, and monitor status, all from a simple API.

## Features

- **REST API server** — Control bots remotely via HTTP using [Gin](https://github.com/gin-gonic/gin)
- **Multi-bot pool** — Spawn and manage multiple named bot instances concurrently
- **Minecraft 1.21.11 support** — Implements protocol 774 with full play-state handling
- **Bot events** — Hook into spawn, chat, system messages, health, death, and disconnect
- **Movement control** — Walk, sprint, and navigate bots to target coordinates
- **Graceful shutdown** — All bots cleanly disconnect on server exit

## Prerequisites

- Go 1.21+
- A running Minecraft Java Edition server (1.21.x, offline mode)

## Setup

**1. Clone the repository**
```sh
git clone https://github.com/deware-pk/go-mcbots.git
cd go-mcbots
```

**2. Copy and configure environment**
```sh
cp .env.example .env
```

| Variable   | Default | Description              |
|------------|---------|--------------------------|
| `PORT`     | `8080`  | HTTP API listening port  |
| `LOG_INFO` | `true`  | Enable structured logging |

**3. Run the API server**
```sh
go run ./cmd/api
```

The server starts on `http://localhost:8080` and also launches a demo bot that connects to `localhost:25565`.

## API Reference

All endpoints accept and return JSON.

---

### Launch a bot

```
POST /bots
```

**Request body**
```json
{
  "id":   "bot-1",
  "name": "GoBot",
  "addr": "localhost:25565"
}
```

**Response** `201 Created`
```json
{ "status": "launched", "id": "bot-1" }
```

---

### List all bots

```
GET /bots
```

**Response** `200 OK`
```json
{ "bots": ["bot-1", "bot-2"], "count": 2 }
```

---

### Remove a bot

```
DELETE /bots/:id
```

**Response** `200 OK`
```json
{ "status": "removed", "id": "bot-1" }
```

---

### Send chat message

```
POST /bots/:id/chat
```

**Request body**
```json
{ "message": "Hello from the API!" }
```

**Response** `200 OK`
```json
{ "status": "sent", "id": "bot-1", "message": "Hello from the API!" }
```

---

### Navigate to coordinates

```
POST /bots/:id/goto
```

**Request body**
```json
{ "x": 100.0, "y": 64.0, "z": -200.0, "sprint": true }
```

**Response** `200 OK`
```json
{
  "status": "navigating",
  "id": "bot-1",
  "target": { "x": 100.0, "y": 64.0, "z": -200.0 }
}
```

---

### Stop movement

```
POST /bots/:id/stop
```

**Response** `200 OK`
```json
{ "status": "stopped", "id": "bot-1" }
```

---

### Get bot status

```
GET /bots/:id/status
```

**Response** `200 OK`
```json
{
  "id": "bot-1",
  "name": "GoBot",
  "connected": true,
  "position": { "x": 100.0, "y": 64.0, "z": -200.0 },
  "health": 20.0,
  "food": 20.0
}
```

---

## Quick Start (Library)

You can also use the bot package directly without the API server:

```go
package main

import (
    "log"
    "time"

    "github.com/deware-pk/go-mcbots/pkg/bot"
    "github.com/deware-pk/go-mcbots/pkg/protocol"
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
        b.SetControlState("forward", true)
        b.SetControlState("sprint", true)
        time.AfterFunc(3*time.Second, func() {
            b.ClearControlStates()
        })
    }

    b.Events.OnChat = func(sender, message string) {
        log.Printf("[Chat] <%s> %s", sender, message)
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

## Upcoming Features

- Advanced pathfinding and world navigation
- Block and entity interaction
- Enhanced autonomous bot behaviors

## Credits

Forked and refactored from:
- [Tnze/go-mc](https://github.com/Tnze/go-mc) — Original Minecraft protocol implementation in Go
- [mj41/go-mc](https://github.com/mj41/go-mc) — Protocol stability improvements

## License

MIT License — see [LICENSE](LICENSE) for details.