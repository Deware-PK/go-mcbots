# go-mcbots

A lightweight Minecraft bot library for Go.
Spin up one bot or hundreds from a single binary — no Node.js, no heavy runtime.

**Use it for:**
- **Server load testing** — simulate many players before a launch or event
- **Plugin testing** — automate join/chat/move scenarios against your plugins
- **Automation & experiments** — build your own headless bots in plain Go

**Features**
- Connect to Minecraft Java Edition servers (offline mode), directly or through
  Velocity / BungeeCord proxies (server switches are followed)
- Client settings (view distance, locale, skin parts, ...) like a real client
- Event hooks: spawn, chat, system messages, health, death, disconnect, pathfinding
- Vanilla movement physics: real block collision shapes (slabs, stairs, fences),
  step-up, swimming, ladders and vines
- A* pathfinding (`GoTo`) that walks stairs and slabs, swims and climbs ladders
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
	ver, _ := bot.ResolveVersion(bot.LatestVersion) // or "1.21.11"
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

## Client settings and proxies

The bot sends its client settings when it joins, like a real client. The
server sends chunks within the bot's view distance (capped by the server's
own `view-distance`), so it decides both how much load a bot puts on chunk
sending and how much memory the bot uses:

```go
b := bot.New("GoBot", ver)
b.SetViewDistance(10) // default 2 (what servers assume without settings)
// or everything at once: b.SetClientSettings(bot.ClientSettings{...})
```

Behind a Velocity or BungeeCord proxy, a server switch (`/server lobby`)
sends the bot back to the configuration phase; it follows automatically,
fires `Events.OnReconfigure`, and `Events.OnSpawn` again in the new server.

## Swarm

```go
ver, _ := bot.ResolveVersion(bot.LatestVersion)
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

| Minecraft | Protocol | Notes |
|-----------|----------|-------|
| 26.2      | 776      | Latest (`bot.LatestVersion`). Tested on Paper. |
| 1.21.11   | 774      | |

Only offline-mode (`online-mode=false`) servers are supported. The examples
take `-version`, e.g. `go run ./examples/testbot -version 1.21.11`.

### Adding a version

Packet IDs and block state IDs are generated from the vanilla server's data
generator, not written by hand:

```bash
java -DbundlerMainClass=net.minecraft.data.Main -jar server.jar --reports --output gen
go run ./internal/tools/genpackets -report gen/reports/packets.json -version 26.2 \
    -out internal/protocol/versions/v776/packets.go -pkg v776
go run ./internal/tools/genblocks -mojang gen/reports/blocks.json \
    -data <minecraft-data>/data/pc/26.1 -mdversion 26.1 -version 26.2 \
    -out internal/protocol/versions/v776/blocks.go -pkg v776
```

`genblocks` also embeds every block state's collision shape (from
PrismarineJS/minecraft-data; new blocks borrow the shape of a vanilla block
of the same Mojang block type).

Packet *layouts* still need checking against the server classes (26.x jars
are not obfuscated, so `javap -p -c` on the packet classes is enough).

## Roadmap

- **Easier API** — simpler, higher-level bot API
- **Minecraft 26.3 support** — same generators as 26.2
- **CI integration tests** — run bots against a real server in CI

## Responsible Use

go-mcbots is intended for testing, automation and learning on servers
you own or have explicit permission to use.

- Do not use it to spam, grief, or flood servers you don't control.
- Respect each server's rules and the Minecraft EULA / Usage Guidelines.
- Running many bots against third-party servers may be treated as a
  denial-of-service attack.

This project is provided "as is" under the MIT License. The authors are
not responsible for any misuse or damage caused by this software.

## Credits

- [Tnze/go-mc](https://github.com/Tnze/go-mc) — original Minecraft protocol implementation in Go. The protocol, NBT and networking code in `internal/protocol` is derived from it; see [NOTICE](NOTICE).
- [mj41/go-mc](https://github.com/mj41/go-mc) — protocol stability improvements

## License

MIT — see [LICENSE](LICENSE). Third-party notices in [NOTICE](NOTICE).
