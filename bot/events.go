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
	OnGoalReached    func()
	OnPathFailed     func(reason string)

	// OnKnockback fires when the server changes our velocity: a Set Entity
	// Velocity for us (the new velocity) or explosion knockback (the amount
	// added). It runs on the network goroutine; the physics applies the
	// change on its next tick.
	OnKnockback func(vx, vy, vz float64)
	// OnExplosion fires for every explosion the server reports.
	OnExplosion func(x, y, z float64, radius float32)

	// OnReconfigure fires when the server takes the bot back to the
	// configuration phase, which proxies (Velocity, BungeeCord) do when
	// switching backend servers. OnSpawn fires again once the bot is in
	// the new world.
	OnReconfigure func()
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
	case "goal_reached":
		if e.OnGoalReached != nil {
			e.OnGoalReached()
		}
	case "knockback":
		if e.OnKnockback != nil && len(args) >= 3 {
			e.OnKnockback(args[0].(float64), args[1].(float64), args[2].(float64))
		}
	case "explosion":
		if e.OnExplosion != nil && len(args) >= 4 {
			e.OnExplosion(args[0].(float64), args[1].(float64), args[2].(float64), args[3].(float32))
		}
	case "reconfigure":
		if e.OnReconfigure != nil {
			e.OnReconfigure()
		}
	case "path_failed":
		if e.OnPathFailed != nil && len(args) >= 1 {
			e.OnPathFailed(args[0].(string))
		}
	}
}
