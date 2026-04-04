package bot

type Events struct {
	OnSpawn          func()
	OnDeath          func()
	OnChat           func(sender, message string)
	OnSystemMessage  func(message string)
	OnPositionUpdate func(x, y, z float64)
	OnHealthUpdate   func(health, food float32)
	OnPhysicsTick    func()
	OnDisconnect     func(reason string)
}

func (e *Events) emit(name string, args ...any) {
	switch name {
	case "spawn":
		if e.OnSpawn != nil {
			e.OnSpawn()
		}
	case "death":
		if e.OnDeath != nil {
			e.OnDeath()
		}
	case "chat":
		if e.OnChat != nil && len(args) >= 2 {
			e.OnChat(args[0].(string), args[1].(string))
		}
	case "system_message":
		if e.OnSystemMessage != nil && len(args) >= 1 {
			e.OnSystemMessage(args[0].(string))
		}
	case "position_update":
		if e.OnPositionUpdate != nil && len(args) >= 3 {
			e.OnPositionUpdate(args[0].(float64), args[1].(float64), args[2].(float64))
		}
	case "health_update":
		if e.OnHealthUpdate != nil && len(args) >= 2 {
			e.OnHealthUpdate(args[0].(float32), args[1].(float32))
		}
	case "physics_tick":
		if e.OnPhysicsTick != nil {
			e.OnPhysicsTick()
		}
	case "disconnect":
		if e.OnDisconnect != nil && len(args) >= 1 {
			e.OnDisconnect(args[0].(string))
		}
	}
}
