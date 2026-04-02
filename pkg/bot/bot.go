package bot

import (
	"go-mcbots/pkg/protocol"
	mcnet "go-mcbots/pkg/protocol/net"
)

type Bot struct {
	conn    *mcnet.Conn
	version protocol.VersionInfo
	Name    string
	OnJoin  func() error
}

func New(name string, version protocol.VersionInfo) *Bot {
	return &Bot{
		Name:    name,
		version: version,
	}
}

func (b *Bot) Connect(addr string) error {
	conn, err := mcnet.DialMC(addr)
	if err != nil {
		return err
	}
	b.conn = conn
	return b.login(addr)
}
