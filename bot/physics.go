package bot

import (
	"sync"
	"time"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

const (
	PhysicsIntervalMs = 50
	Gravity           = 0.08
	TerminalVelocity  = -3.92
	PlayerWidth       = 0.6
	PlayerHeight      = 1.8
)

type ControlState struct {
	Forward bool
	Back    bool
	Left    bool
	Right   bool
	Jump    bool
	Sprint  bool
	Sneak   bool
}

type Physics struct {
	bot     *Bot
	mu      sync.RWMutex
	control ControlState
	running bool
	stopCh  chan struct{}

	lastSentX, lastSentY, lastSentZ float64
	lastSentYaw, lastSentPitch      float32
	lastSentOnGround                bool
	lastSentTime                    time.Time
	shouldSendPosition              bool
}

func newPhysics(bot *Bot) *Physics {
	return &Physics{
		bot:    bot,
		stopCh: make(chan struct{}),
	}
}

func (p *Physics) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.shouldSendPosition = true

	x, y, z := p.bot.state.GetPosition()
	p.lastSentX, p.lastSentY, p.lastSentZ = x, y, z
	yaw, pitch := p.bot.state.GetRotation()
	p.lastSentYaw, p.lastSentPitch = yaw, pitch
	p.lastSentOnGround = p.bot.state.IsOnGround()
	p.lastSentTime = time.Now()
	p.mu.Unlock()

	go p.tickLoop(p.stopCh)
}

func (p *Physics) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)
}

// SetControlState sets one control. Changes are sent to the server as a
// Player Input packet; sprint changes also send a Player Command.
func (p *Physics) SetControlState(control string, state bool) {
	p.mu.Lock()
	old := p.control
	switch control {
	case "forward":
		p.control.Forward = state
	case "back":
		p.control.Back = state
	case "left":
		p.control.Left = state
	case "right":
		p.control.Right = state
	case "jump":
		p.control.Jump = state
	case "sprint":
		p.control.Sprint = state
	case "sneak":
		p.control.Sneak = state
	}
	cur := p.control
	p.mu.Unlock()

	p.sendControlChanges(old, cur)
}

func (p *Physics) GetControlState(control string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch control {
	case "forward":
		return p.control.Forward
	case "back":
		return p.control.Back
	case "left":
		return p.control.Left
	case "right":
		return p.control.Right
	case "jump":
		return p.control.Jump
	case "sprint":
		return p.control.Sprint
	case "sneak":
		return p.control.Sneak
	}
	return false
}

func (p *Physics) ClearControlStates() {
	p.mu.Lock()
	old := p.control
	p.control = ControlState{}
	p.mu.Unlock()

	p.sendControlChanges(old, ControlState{})
}

// sendControlChanges tells the server about control changes between old and cur.
func (p *Physics) sendControlChanges(old, cur ControlState) {
	if old.Sprint != cur.Sprint {
		if cur.Sprint {
			p.bot.sendPlayerCommand(playerCommandStartSprinting)
		} else {
			p.bot.sendPlayerCommand(playerCommandStopSprinting)
		}
	}
	if old != cur {
		p.bot.sendPlayerInput(cur)
	}
}

func (p *Physics) tickLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(PhysicsIntervalMs * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

func (p *Physics) tick() {
	// No movement between Respawn and the following position sync.
	if !p.bot.state.IsAlive() || p.bot.awaitingSpawn.Load() {
		return
	}

	p.bot.nav.Tick()
	p.simulatePlayer()
	p.updatePosition()
	p.bot.Events.emit("physics_tick")
}

func (p *Physics) simulatePlayer() {
	p.mu.RLock()
	ctrl := p.control
	p.mu.RUnlock()

	x, y, z := p.bot.state.GetPosition()
	vx, vy, vz := p.bot.state.GetVelocity()
	yaw, _ := p.bot.state.GetRotation()

	next := stepPhysics(p.bot.world, body{
		X: x, Y: y, Z: z,
		VX: vx, VY: vy, VZ: vz,
		OnGround: p.bot.state.IsOnGround(),
	}, ctrl, yaw)

	p.bot.state.SetPosition(next.X, next.Y, next.Z)
	p.bot.state.SetVelocity(next.VX, next.VY, next.VZ)
	p.bot.state.SetOnGround(next.OnGround)
}

func (p *Physics) updatePosition() {
	if p.bot.awaitingSpawn.Load() {
		return // a respawn arrived during this tick
	}
	x, y, z := p.bot.state.GetPosition()
	yaw, pitch := p.bot.state.GetRotation()
	onGround := p.bot.state.IsOnGround()

	p.mu.RLock()
	posChanged := p.lastSentX != x || p.lastSentY != y || p.lastSentZ != z ||
		time.Since(p.lastSentTime) >= time.Second
	lookChanged := p.lastSentYaw != yaw || p.lastSentPitch != pitch
	groundChanged := p.lastSentOnGround != onGround
	p.mu.RUnlock()

	var err error
	if posChanged && lookChanged {
		err = p.bot.sendPositionAndRotation()
	} else if posChanged {
		err = p.bot.sendPosition()
	} else if lookChanged {
		err = p.bot.sendRotation()
	} else if groundChanged {
		err = p.bot.sendOnGround()
	}

	if err != nil {
		return
	}

	p.mu.Lock()
	if posChanged {
		p.lastSentX, p.lastSentY, p.lastSentZ = x, y, z
		p.lastSentTime = time.Now()
	}
	if lookChanged {
		p.lastSentYaw, p.lastSentPitch = yaw, pitch
	}
	p.lastSentOnGround = onGround
	p.mu.Unlock()
}

// Player Command action IDs (protocol 774; enum since 1.21.2).
const (
	playerCommandStopSleeping    = 0
	playerCommandStartSprinting  = 1
	playerCommandStopSprinting   = 2
	playerCommandStartRidingJump = 3
	playerCommandStopRidingJump  = 4
	playerCommandOpenInventory   = 5
	playerCommandStartFallFlying = 6
)

// Player Input flags (protocol 774).
const (
	inputForward  = 0x01
	inputBackward = 0x02
	inputLeft     = 0x04
	inputRight    = 0x08
	inputJump     = 0x10
	inputSneak    = 0x20
	inputSprint   = 0x40
)

func (c ControlState) inputFlags() byte {
	var f byte
	for _, x := range []struct {
		on   bool
		flag byte
	}{
		{c.Forward, inputForward}, {c.Back, inputBackward}, {c.Left, inputLeft},
		{c.Right, inputRight}, {c.Jump, inputJump}, {c.Sneak, inputSneak}, {c.Sprint, inputSprint},
	} {
		if x.on {
			f |= x.flag
		}
	}
	return f
}

func (b *Bot) sendPlayerInput(c ControlState) error {
	if b.conn == nil {
		return nil
	}
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerInput),
		pk.UnsignedByte(c.inputFlags()),
	))
}

func (b *Bot) sendPlayerCommand(actionID int32) error {
	if b.conn == nil {
		return nil
	}
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerCommand),
		pk.VarInt(b.state.EntityID),
		pk.VarInt(actionID),
		pk.VarInt(0),
	))
}
