// Command testbot joins a server and obeys chat commands, for manual testing
// of respawn, physics and pathfinding against a real server.
//
//	go run ./examples/testbot -addr localhost:25565 -name TestBot
//
// Chat commands (type them in game chat):
//
//	!pos               report position and health
//	!goto X Y Z        walk to block coordinates (sprinting)
//	!walk X Y Z        same, without sprint
//	!stop              stop moving
//	!sprint on|off     toggle sprint (other players should see particles)
//	!sneak on|off      toggle sneak (other players should see crouching)
//	!jump              jump once
//	!forward SECONDS   walk forward for N seconds
//
// The bot respawns automatically 1 second after dying.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/deware-pk/go-mcbots/bot"
)

func main() {
	addr := flag.String("addr", "localhost:25565", "server address (offline mode)")
	version := flag.String("version", bot.LatestVersion, "Minecraft version of the server (see bot.SupportedVersions)")
	name := flag.String("name", "TestBot", "bot username")
	flag.Parse()

	ver, err := bot.ResolveVersion(*version)
	if err != nil {
		log.Fatal(err)
	}

	b := bot.New(*name, ver)
	spawns := 0

	b.Events.OnSpawn = func() {
		spawns++
		x, y, z := b.GetPosition()
		log.Printf("[spawn #%d] at %.1f %.1f %.1f", spawns, x, y, z)
		b.Chat(fmt.Sprintf("spawned (#%d) at %.0f %.0f %.0f", spawns, x, y, z))
	}
	b.Events.OnDeath = func() {
		log.Println("[death] respawning in 1s")
		time.AfterFunc(time.Second, func() {
			if err := b.Respawn(); err != nil {
				log.Printf("respawn error: %v", err)
			}
		})
	}
	b.Events.OnGoalReached = func() {
		log.Println("[path] goal reached")
		b.Chat("arrived")
	}
	b.Events.OnPathFailed = func(reason string) {
		log.Printf("[path] failed: %s", reason)
		b.Chat("path failed: " + reason)
	}
	b.Events.OnSystemMessage = func(message string) {
		log.Printf("[system] %s", message)
	}
	b.Events.OnDisconnect = func(reason string) {
		log.Printf("[disconnect] %s", reason)
	}
	b.Events.OnChat = func(sender, message string) {
		log.Printf("<%s> %s", sender, message)
		if sender == *name || !strings.HasPrefix(message, "!") {
			return
		}
		go handleCommand(b, strings.Fields(message))
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		b.Close()
	}()

	if err := b.Connect(*addr); err != nil {
		log.Printf("connection ended: %v", err)
	}
}

func handleCommand(b *bot.Bot, args []string) {
	switch args[0] {
	case "!pos":
		x, y, z := b.GetPosition()
		hp, food := b.GetHealth()
		b.Chat(fmt.Sprintf("pos %.2f %.2f %.2f | hp %.0f food %.0f | ground %v",
			x, y, z, hp, food, b.IsOnGround()))

	case "!goto", "!walk":
		if len(args) != 4 {
			b.Chat("usage: " + args[0] + " X Y Z")
			return
		}
		var c [3]float64
		for i := 0; i < 3; i++ {
			v, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				b.Chat("bad number: " + args[i+1])
				return
			}
			c[i] = v
		}
		if err := b.GoTo(c[0], c[1], c[2], args[0] == "!goto"); err != nil {
			b.Chat("goto error: " + err.Error())
			return
		}
		b.Chat(fmt.Sprintf("going to %.0f %.0f %.0f", c[0], c[1], c[2]))

	case "!stop":
		b.StopPathfinding()
		b.ClearControlStates()
		b.Chat("stopped")

	case "!sprint", "!sneak":
		if len(args) != 2 {
			b.Chat("usage: " + args[0] + " on|off")
			return
		}
		b.SetControlState(strings.TrimPrefix(args[0], "!"), args[1] == "on")
		b.Chat(args[0][1:] + " " + args[1])

	case "!jump":
		b.SetControlState("jump", true)
		time.Sleep(100 * time.Millisecond)
		b.SetControlState("jump", false)

	case "!forward":
		secs := 2.0
		if len(args) == 2 {
			if v, err := strconv.ParseFloat(args[1], 64); err == nil {
				secs = v
			}
		}
		b.SetControlState("forward", true)
		time.Sleep(time.Duration(secs * float64(time.Second)))
		b.SetControlState("forward", false)

	default:
		b.Chat("commands: !pos !goto !walk !stop !sprint !sneak !jump !forward")
	}
}
