package bot

import "sync"

type State struct {
	mu sync.RWMutex

	X, Y, Z    float64
	Yaw, Pitch float32

	VelX, VelY, VelZ float64

	OnGround bool

	Health         float32
	Food           float32
	FoodSaturation float32

	EntityID int32
	Alive    bool
}

func newState() *State {
	return &State{
		OnGround: true,
		Alive:    true,
	}
}

func (s *State) GetPosition() (x, y, z float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.X, s.Y, s.Z
}

func (s *State) SetPosition(x, y, z float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.X, s.Y, s.Z = x, y, z
}

func (s *State) GetRotation() (yaw, pitch float32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Yaw, s.Pitch
}

func (s *State) SetRotation(yaw, pitch float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Yaw, s.Pitch = yaw, pitch
}

func (s *State) GetVelocity() (vx, vy, vz float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.VelX, s.VelY, s.VelZ
}

func (s *State) SetVelocity(vx, vy, vz float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VelX, s.VelY, s.VelZ = vx, vy, vz
}

func (s *State) GetHealth() (health, food float32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Health, s.Food
}

func (s *State) SetHealth(health, food, saturation float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Health, s.Food, s.FoodSaturation = health, food, saturation
}

func (s *State) IsAlive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Alive
}

func (s *State) SetAlive(alive bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Alive = alive
}

func (s *State) IsOnGround() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.OnGround
}

func (s *State) SetOnGround(onGround bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OnGround = onGround
}
