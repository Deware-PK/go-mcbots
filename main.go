package main

import (
	"go-mcbots/pkg/bot"
	"go-mcbots/pkg/protocol"
	"log"
)

func main() {
	ver, err := protocol.Resolve("1.21.11")
	if err != nil {
		log.Fatal(err)
	}

	b := bot.New("GoFriendBot1", ver)
	b.OnJoin = func() error {
		return b.Chat("hello from bot!")
	}
	if err := b.Connect("localhost:25565"); err != nil {
		log.Fatal(err)
	}

	log.Println("login success — Configuration phase")
}
