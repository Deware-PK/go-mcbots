# go-mcbots

A lightweight Minecraft bot library for Go.

- Connect bots to Minecraft Java Edition servers (offline mode)
- Event hooks: spawn, chat, system messages, health, death, disconnect, pathfinding
- Movement controls, physics and A* pathfinding (`GoTo`)
- `swarm` package to run many bots at once

## Install

Requires **Go 1.25+**.

```bash
go get github.com/deware-pk/go-mcbots
```

## Quick start

```go
package main

import (
	"log"

	"github.com/deware-pk/go-mcbots/bot"
)

func main() {
	ver, _ := bot.ResolveVersion("1.21.11")
	b := bot.New("GoBot", ver)
	b.Events.OnSpawn = func() { b.Chat("Hello from go-mcbots!") }
	b.Events.OnChat = func(sender, msg string) { log.Printf("<%s> %s", sender, msg) }
	log.Println(b.Connect("localhost:25565")) // blocks until disconnect
}
```

Full version with flags and Ctrl-C handling: [examples/hello](examples/hello/main.go).

```bash
go run ./examples/hello -addr localhost:25565 -name GoBot
```

## Swarm

```go
ver, _ := bot.ResolveVersion("1.21.11")
s := swarm.New()
for i := 1; i <= 10; i++ {
	name := fmt.Sprintf("Bot_%d", i)
	s.Launch(name, name, ver, "localhost:25565", func(b *bot.Bot) {
		b.Events.OnSpawn = func() { log.Println(name, "spawned") }
	})
}
defer s.Shutdown()
```

See [examples/swarm](examples/swarm/main.go):

```bash
go run ./examples/swarm -addr localhost:25565 -n 10 -prefix Bot_
```

## Supported versions

| Minecraft | Protocol |
|-----------|----------|
| 1.21.11   | 774      |

Only offline-mode (`online-mode=false`) servers are supported.

## Roadmap

- **Easier API** — simpler, higher-level bot API
- **Minecraft 26.1 support** — via generated protocol data instead of hand-written packet tables
- **CI integration tests** — run bots against a real server in CI

## Credits

- [Tnze/go-mc](https://github.com/Tnze/go-mc) — original Minecraft protocol implementation in Go. The protocol, NBT and networking code in `internal/protocol` is derived from it; see [NOTICE](NOTICE).
- [mj41/go-mc](https://github.com/mj41/go-mc) — protocol stability improvements

## License

MIT — see [LICENSE](LICENSE). Third-party notices in [NOTICE](NOTICE).
