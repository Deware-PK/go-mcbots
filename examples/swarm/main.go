// Command swarm launches N bots named <prefix><i> against one server.
//
//	go run ./examples/swarm -addr localhost:25565 -n 10 -prefix Bot_
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/deware-pk/go-mcbots/bot"
	"github.com/deware-pk/go-mcbots/swarm"
)

func main() {
	addr := flag.String("addr", "localhost:25565", "server address (offline mode)")
	version := flag.String("version", bot.LatestVersion, "Minecraft version of the server (see bot.SupportedVersions)")
	n := flag.Int("n", 5, "number of bots")
	prefix := flag.String("prefix", "Bot_", "username prefix")
	delay := flag.Duration("delay", 500*time.Millisecond, "delay between joins")
	flag.Parse()

	ver, err := bot.ResolveVersion(*version)
	if err != nil {
		log.Fatal(err)
	}

	s := swarm.New()
	for i := 1; i <= *n; i++ {
		name := fmt.Sprintf("%s%d", *prefix, i)
		setup := func(b *bot.Bot) {
			b.Events.OnSpawn = func() {
				log.Printf("%s spawned", name)
			}
			b.Events.OnDisconnect = func(reason string) {
				log.Printf("%s disconnected: %s", name, reason)
			}
		}
		if err := s.Launch(name, name, ver, *addr, setup); err != nil {
			log.Printf("launch %s: %v", name, err)
		}
		time.Sleep(*delay)
	}

	// Run until Ctrl-C, then disconnect every bot.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	s.Shutdown()
}
