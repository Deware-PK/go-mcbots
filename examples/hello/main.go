// Command hello connects a single bot, says hello on spawn and logs chat.
//
//	go run ./examples/hello -addr localhost:25565 -name GoBot
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/deware-pk/go-mcbots/bot"
)

func main() {
	addr := flag.String("addr", "localhost:25565", "server address (offline mode)")
	name := flag.String("name", "GoBot", "bot username")
	flag.Parse()

	ver, err := bot.ResolveVersion("1.21.11")
	if err != nil {
		log.Fatal(err)
	}

	b := bot.New(*name, ver)
	b.Events.OnSpawn = func() {
		x, y, z := b.GetPosition()
		log.Printf("spawned at %.1f %.1f %.1f", x, y, z)
		b.Chat("Hello from go-mcbots!")
	}
	b.Events.OnChat = func(sender, message string) {
		log.Printf("<%s> %s", sender, message)
	}
	b.Events.OnSystemMessage = func(message string) {
		log.Printf("[system] %s", message)
	}
	b.Events.OnDisconnect = func(reason string) {
		log.Printf("disconnected: %s", reason)
	}

	// Close the connection on Ctrl-C; Connect then returns.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		b.Close()
	}()

	// Connect blocks until the bot disconnects.
	if err := b.Connect(*addr); err != nil {
		log.Printf("connection ended: %v", err)
	}
}
