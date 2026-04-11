package bot

import (
	"sync"

	"github.com/deware-pk/go-mcbots/pkg/bot/pathfinder"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
	mcnet "github.com/deware-pk/go-mcbots/pkg/protocol/net"
	pk "github.com/deware-pk/go-mcbots/pkg/protocol/net/packet"
)

type Bot struct {
	conn    *mcnet.Conn
	writeMu sync.Mutex
	version protocol.VersionInfo
	state   *State
	world   *World
	physics *Physics
	nav     *pathfinder.Pathfinder

	Name    string
	Events  Events
	onClose func()
}

func (b *Bot) SetOnClose(fn func()) {
	b.onClose = fn
}

func (b *Bot) writePacket(p pk.Packet) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.conn.WritePacket(p)
}

func New(name string, version protocol.VersionInfo) *Bot {
	b := &Bot{
		Name:    name,
		version: version,
		state:   newState(),
	}
	b.world = newWorld()
	b.physics = newPhysics(b)
	b.nav = pathfinder.New(b, b.world)
	b.nav.SetCallbacks(
		func() { b.Events.emit("goal_reached") },
		func(reason string) { b.Events.emit("path_failed", reason) },
	)
	return b
}

func (b *Bot) Connect(addr string) error {
	conn, err := mcnet.DialMC(addr)
	if err != nil {
		return err
	}
	b.conn = conn
	return b.login(addr)
}

func (b *Bot) Close() error {
	b.physics.Stop()
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

func (b *Bot) GetPosition() (x, y, z float64) {
	return b.state.GetPosition()
}

func (b *Bot) GetRotation() (yaw, pitch float32) {
	return b.state.GetRotation()
}

func (b *Bot) GetHealth() (health, food float32) {
	return b.state.GetHealth()
}

func (b *Bot) IsAlive() bool {
	return b.state.IsAlive()
}

func (b *Bot) IsOnGround() bool {
	return b.state.IsOnGround()
}

func (b *Bot) SetControlState(control string, state bool) {
	b.physics.SetControlState(control, state)
}

func (b *Bot) GetControlState(control string) bool {
	return b.physics.GetControlState(control)
}

func (b *Bot) ClearControlStates() {
	b.physics.ClearControlStates()
}

func (b *Bot) GetBlock(x, y, z int) uint32 {
	return b.world.GetBlock(x, y, z)
}

func (b *Bot) GoTo(x, y, z float64, sprint bool) error {
	return b.nav.GoTo(x, y, z, sprint)
}

func (b *Bot) StopPathfinding() {
	b.nav.Stop()
}

func (b *Bot) IsNavigating() bool {
	return b.nav.IsNavigating()
}

func (b *Bot) GetPathProgress() (current, total int) {
	return b.nav.GetProgress()
}

func (b *Bot) GetWorldView() WorldView {
	return b.world
}
