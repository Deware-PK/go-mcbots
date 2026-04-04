package bot

import (
	"math"
	"sync"
	"time"

	pk "go-mcbots/pkg/protocol/net/packet"
)

const (
	PhysicsIntervalMs = 50
	Gravity           = 0.08
	Drag              = 0.02
	WalkSpeed         = 0.1
	SprintSpeed       = 0.13
	SneakSpeed        = 0.03
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

	go p.tickLoop()
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

func (p *Physics) SetControlState(control string, state bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
		if state {
			p.bot.sendPlayerCommand(3) // start sprinting
		} else {
			p.bot.sendPlayerCommand(4) // stop sprinting
		}
	case "sneak":
		p.control.Sneak = state
		if state {
			p.bot.sendPlayerCommand(0) // start sneaking
		} else {
			p.bot.sendPlayerCommand(1) // stop sneaking
		}
	}
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
	defer p.mu.Unlock()
	p.control = ControlState{}
}

func (p *Physics) tickLoop() {
	ticker := time.NewTicker(PhysicsIntervalMs * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

func (p *Physics) tick() {
	if !p.bot.state.IsAlive() {
		return
	}

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
	onGround := p.bot.state.IsOnGround()

	yawRad := float64(yaw) * math.Pi / 180.0

	var moveX, moveZ float64
	if ctrl.Forward {
		moveX -= math.Sin(yawRad)
		moveZ += math.Cos(yawRad)
	}
	if ctrl.Back {
		moveX += math.Sin(yawRad)
		moveZ -= math.Cos(yawRad)
	}
	if ctrl.Left {
		moveX += math.Cos(yawRad)
		moveZ += math.Sin(yawRad)
	}
	if ctrl.Right {
		moveX -= math.Cos(yawRad)
		moveZ -= math.Sin(yawRad)
	}

	length := math.Sqrt(moveX*moveX + moveZ*moveZ)
	if length > 0 {
		moveX /= length
		moveZ /= length
	}

	speed := WalkSpeed
	if ctrl.Sprint {
		speed = SprintSpeed
	}
	if ctrl.Sneak {
		speed = SneakSpeed
	}

	if onGround {
		vx = moveX * speed
		vz = moveZ * speed

		if ctrl.Jump {
			vy = 0.42
		}
	} else {
		vx += moveX * 0.02
		vz += moveZ * 0.02
	}

	vy -= Gravity
	if vy < TerminalVelocity {
		vy = TerminalVelocity
	}

	vx *= (1 - Drag)
	vz *= (1 - Drag)
	vy *= 0.98

	newX := x + vx
	newY := y + vy
	newZ := z + vz

	newOnGround := false

	blockBelowY := int(math.Floor(newY - 0.01))
	blockX := int(math.Floor(newX))
	blockZ := int(math.Floor(newZ))

	if p.bot.world.IsBlockSolidOrUnloaded(blockX, blockBelowY, blockZ) {
		groundY := float64(blockBelowY + 1)
		if newY < groundY {
			newY = groundY
			vy = 0
			newOnGround = true
		}
	}

	halfW := PlayerWidth / 2.0
	checkPositions := [][2]int{
		{int(math.Floor(newX - halfW)), int(math.Floor(newZ - halfW))},
		{int(math.Floor(newX + halfW)), int(math.Floor(newZ - halfW))},
		{int(math.Floor(newX - halfW)), int(math.Floor(newZ + halfW))},
		{int(math.Floor(newX + halfW)), int(math.Floor(newZ + halfW))},
	}

	feetY := int(math.Floor(newY))
	headY := int(math.Floor(newY + PlayerHeight))
	for checkY := feetY; checkY <= headY; checkY++ {
		for _, cp := range checkPositions {
			if p.bot.world.IsBlockSolid(cp[0], checkY, cp[1]) {
				newX = x
				newZ = z
				vx = 0
				vz = 0
				goto doneCollision
			}
		}
	}
doneCollision:

	p.bot.state.SetPosition(newX, newY, newZ)
	p.bot.state.SetVelocity(vx, vy, vz)
	p.bot.state.SetOnGround(newOnGround)
}

func (p *Physics) updatePosition() {
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
