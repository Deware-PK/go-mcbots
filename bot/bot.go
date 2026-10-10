package bot

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/deware-pk/go-mcbots/bot/pathfinder"
	"github.com/deware-pk/go-mcbots/internal/protocol"
	mcnet "github.com/deware-pk/go-mcbots/internal/protocol/net"
	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

type Bot struct {
	conn    *mcnet.Conn
	writeMu sync.Mutex
	version protocol.VersionInfo
	state   *State
	world   *World
	physics *Physics
	nav     *pathfinder.Pathfinder

	// awaitingSpawn is true from join/respawn until the first Synchronize
	// Player Position; physics must not send movement in between.
	awaitingSpawn atomic.Bool
	// dimTypes is the minecraft:dimension_type registry from configuration.
	dimTypes []dimensionType

	// phase is the connection state (phaseLogin, phaseConfig, phasePlay).
	// It changes only while writeMu is held, so a play packet can never
	// follow Acknowledge Configuration on the wire.
	phase atomic.Int32

	settingsMu sync.Mutex
	settings   ClientSettings

	Name    string
	Events  Events
	onClose func()
}

func (b *Bot) SetOnClose(fn func()) {
	b.onClose = fn
}

// Connection phases.
const (
	phaseLogin int32 = iota
	phaseConfig
	phasePlay
)

// ErrNotInPlay is returned when a play packet (chat, movement, ...) is sent
// while the bot is logging in or reconfiguring, e.g. during a proxy server
// switch. The packet is dropped.
var ErrNotInPlay = errors.New("not in play state")

// writePacket sends a play-state packet. It is dropped with ErrNotInPlay
// outside the play state.
func (b *Bot) writePacket(p pk.Packet) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if b.phase.Load() != phasePlay {
		return ErrNotInPlay
	}
	return b.conn.WritePacket(p)
}

// writeRaw sends a packet regardless of the connection phase (login and
// configuration packets).
func (b *Bot) writeRaw(p pk.Packet) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.conn.WritePacket(p)
}

// setPhase changes the connection phase. If p is not nil it is written
// first, atomically with the change.
func (b *Bot) setPhase(phase int32, p *pk.Packet) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if p != nil {
		if err := b.conn.WritePacket(*p); err != nil {
			return err
		}
	}
	b.phase.Store(phase)
	return nil
}

func New(name string, version Version) *Bot {
	b := &Bot{
		Name:     name,
		version:  version,
		state:    newState(),
		settings: DefaultClientSettings(),
	}
	b.awaitingSpawn.Store(true)
	b.world = newWorld(version.BlockClasses, version.BlockShapes)
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
	b.awaitingSpawn.Store(true)
	b.phase.Store(phaseLogin)
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

// IsInWater reports whether the bot touched water on the last physics tick.
func (b *Bot) IsInWater() bool {
	return b.physics.contactState().InWater
}

// IsOnClimbable reports whether the bot is on a ladder, vine or other
// climbable block.
func (b *Bot) IsOnClimbable() bool {
	return b.physics.contactState().OnClimbable
}

// IsCollidedHorizontally reports whether the bot walked into something on
// the last physics tick.
func (b *Bot) IsCollidedHorizontally() bool {
	return b.physics.contactState().HorizontalCollision
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
