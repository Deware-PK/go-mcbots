package bot

import (
	"fmt"
	"log"
	"net"
	"strconv"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

func (b *Bot) login(addr string) error {

	if err := b.sendHandshake(addr); err != nil {
		return err
	}

	if err := b.sendLoginStart(); err != nil {
		return err
	}

	return b.waitLoginSuccess()
}

// handshakeTarget splits addr into the host and port sent in the handshake.
// Proxies use the host for forced hosts, so it must not contain the port.
func handshakeTarget(addr string) (string, uint16) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 25565 // no port in addr
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return host, 25565
	}
	return host, uint16(port)
}

func (b *Bot) sendHandshake(addr string) error {
	host, port := handshakeTarget(addr)
	return b.writeRaw(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_Handshake),
		pk.VarInt(b.version.ProtocolNumber),
		pk.String(host),
		pk.UnsignedShort(port),
		pk.VarInt(2),
	))
}

func (b *Bot) sendLoginStart() error {
	uuid := offlineUUID(b.Name)
	return b.writeRaw(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_LoginStart),
		pk.String(b.Name),
		pk.UUID(uuid),
	))
}

func (b *Bot) waitLoginSuccess() error {
	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			return fmt.Errorf("login: %w", err)
		}

		switch p.ID {

		case b.version.IDs.CB_SetCompression:
			var threshold pk.VarInt
			if err := p.Scan(&threshold); err != nil {
				return err
			}
			b.conn.SetThreshold(int(threshold))

		case b.version.IDs.CB_LoginSuccess:
			ack := pk.Marshal(pk.VarInt(b.version.IDs.SB_LoginAck))
			if err := b.setPhase(phaseConfig, &ack); err != nil {
				return err
			}
			return b.handleConfiguration()

		case b.version.IDs.CB_LoginPluginRequest:
			// Login Plugin Request: VarInt message ID, Identifier channel,
			// data. We understand no channel: answer "not understood".
			var msgID pk.VarInt
			var channel pk.String
			if err := p.Scan(&msgID, &channel); err != nil {
				return fmt.Errorf("login plugin request: %w", err)
			}
			if err := b.writeRaw(pk.Marshal(
				pk.VarInt(b.version.IDs.SB_LoginPluginResponse),
				msgID,
				pk.Boolean(false),
			)); err != nil {
				return err
			}

		case b.version.IDs.CB_CookieRequest_Login:
			if err := b.answerCookieRequest(p, b.version.IDs.SB_CookieResponse_Login, b.writeRaw); err != nil {
				return err
			}

		case b.version.IDs.CB_Disconnect:
			// Login-state disconnect: the reason is a JSON text component.
			var reason pk.String
			p.Scan(&reason)
			b.Events.emit("disconnect", string(reason))
			return fmt.Errorf("disconnected during login: %s", reason)

		default:
			log.Printf("[Login] unhandled packet 0x%02X", p.ID)
		}
	}
}

// handleConfiguration runs the first configuration phase after login, then
// the game loop.
func (b *Bot) handleConfiguration() error {
	if err := b.configure(); err != nil {
		return err
	}
	return b.HandleGame()
}

// configure runs one configuration phase, from the first configuration
// packet to Finish Configuration. It runs after login and again every time
// the server sends Start Configuration (e.g. a proxy switching servers).
func (b *Bot) configure() error {
	// The vanilla client announces its settings and brand first.
	if err := b.sendClientInformation(); err != nil {
		return err
	}
	if err := b.sendBrand(); err != nil {
		return err
	}

	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			b.Events.emit("disconnect", err.Error())
			return fmt.Errorf("configuration: %w", err)
		}

		switch p.ID {

		case b.version.IDs.CB_KnownPacks:
			if err := b.writeRaw(pk.Marshal(
				pk.VarInt(b.version.IDs.SB_KnownPacks),
				pk.VarInt(1),
				pk.String("minecraft"),
				pk.String("core"),
				pk.String(b.version.MCVersion),
			)); err != nil {
				return err
			}

		case b.version.IDs.CB_RegistryData:
			id, dims, err := parseRegistryData(p.Data)
			if err != nil {
				log.Printf("[World] registry data %q: %v", id, err)
			} else if id == "minecraft:dimension_type" {
				b.dimTypes = dims
			}

		case b.version.IDs.CB_PluginRequest:
			// intentionally ignored

		case b.version.IDs.CB_FinishConfig:
			fin := pk.Marshal(pk.VarInt(b.version.IDs.SB_FinishConfig))
			return b.setPhase(phasePlay, &fin)

		case b.version.IDs.CB_FeatureFlags:
			// Ignored

		case b.version.IDs.CB_KeepAlive_Config:
			var id pk.Long
			if err := p.Scan(&id); err != nil {
				return fmt.Errorf("config keep-alive: %w", err)
			}
			if err := b.writeRaw(pk.Marshal(pk.VarInt(b.version.IDs.SB_KeepAlive_Config), id)); err != nil {
				return err
			}

		case b.version.IDs.CB_Ping_Config:
			var id pk.Int
			if err := p.Scan(&id); err != nil {
				return fmt.Errorf("config ping: %w", err)
			}
			if err := b.writeRaw(pk.Marshal(pk.VarInt(b.version.IDs.SB_Pong_Config), id)); err != nil {
				return err
			}

		case b.version.IDs.CB_ResourcePackPush_Config:
			if err := b.answerResourcePack(p, b.version.IDs.SB_ResourcePack_Config, b.writeRaw); err != nil {
				return err
			}

		case b.version.IDs.CB_CookieRequest_Config:
			if err := b.answerCookieRequest(p, b.version.IDs.SB_CookieResponse_Config, b.writeRaw); err != nil {
				return err
			}

		case b.version.IDs.CB_CodeOfConduct:
			// The server's code of conduct must be accepted to join.
			if err := b.writeRaw(pk.Marshal(pk.VarInt(b.version.IDs.SB_AcceptCodeOfConduct))); err != nil {
				return err
			}

		case b.version.IDs.CB_Disconnect_Config:
			reason := readDisconnectReason(p)
			b.Events.emit("disconnect", reason)
			return fmt.Errorf("disconnected during config: %s", reason)

		default:
			// Tags, server links, report details, dialogs, ...: not needed.
		}
	}
}

// startConfiguration handles a play-state Start Configuration: the server
// (usually a proxy switching backend servers) takes the client back to the
// configuration phase. Movement stops until the next spawn.
func (b *Bot) startConfiguration() error {
	b.awaitingSpawn.Store(true)
	b.physics.Stop()
	b.nav.Stop()
	b.physics.resetControls()

	ack := pk.Marshal(pk.VarInt(b.version.IDs.SB_ConfigurationAck))
	if err := b.setPhase(phaseConfig, &ack); err != nil {
		return err
	}
	b.Events.emit("reconfigure")
	return b.configure()
}

// answerResourcePack accepts a resource pack push (it is never downloaded):
// Accepted, Downloaded, then Successfully loaded, like a client that has it.
func (b *Bot) answerResourcePack(p pk.Packet, responseID int32, write func(pk.Packet) error) error {
	var id pk.UUID
	if err := p.Scan(&id); err != nil {
		return fmt.Errorf("resource pack push: %w", err)
	}
	const (
		packLoaded     = 0
		packAccepted   = 3
		packDownloaded = 4
	)
	for _, status := range []int32{packAccepted, packDownloaded, packLoaded} {
		if err := write(pk.Marshal(pk.VarInt(responseID), id, pk.VarInt(status))); err != nil {
			return err
		}
	}
	return nil
}

// answerCookieRequest replies that we have no cookie for the requested key.
func (b *Bot) answerCookieRequest(p pk.Packet, responseID int32, write func(pk.Packet) error) error {
	var key pk.String
	if err := p.Scan(&key); err != nil {
		return fmt.Errorf("cookie request: %w", err)
	}
	return write(pk.Marshal(pk.VarInt(responseID), key, pk.Boolean(false)))
}

// sendBrand sends the minecraft:brand plugin message like the vanilla client.
func (b *Bot) sendBrand() error {
	return b.writeRaw(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PluginResponse),
		pk.String("minecraft:brand"),
		pk.String("vanilla"),
	))
}
