package bot

import (
	"fmt"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

// ChatMode is the chat visibility the client asks for.
type ChatMode int32

const (
	ChatFull         ChatMode = 0 // all messages
	ChatCommandsOnly ChatMode = 1 // command feedback only
	ChatHidden       ChatMode = 2 // nothing
)

// Hand is a main hand setting.
type Hand int32

const (
	HandLeft  Hand = 0
	HandRight Hand = 1
)

// ParticleStatus is the client particle setting.
type ParticleStatus int32

const (
	ParticlesAll       ParticleStatus = 0
	ParticlesDecreased ParticleStatus = 1
	ParticlesMinimal   ParticleStatus = 2
)

// Skin part bits for ClientSettings.SkinParts.
const (
	SkinCape        byte = 0x01
	SkinJacket      byte = 0x02
	SkinLeftSleeve  byte = 0x04
	SkinRightSleeve byte = 0x08
	SkinLeftPants   byte = 0x10
	SkinRightPants  byte = 0x20
	SkinHat         byte = 0x40
	SkinAll         byte = 0x7F
)

// ClientSettings is what the bot reports in the Client Information packet,
// like the options screen of a real client.
//
// ViewDistance decides how many chunks the server sends: the server uses
// the smaller of this and its own view-distance. More chunks means more
// load on the server (closer to real players) and more memory in the bot.
type ClientSettings struct {
	Locale             string // e.g. "en_us"
	ViewDistance       int    // in chunks, 2..32
	ChatMode           ChatMode
	ChatColors         bool
	SkinParts          byte // SkinCape | SkinHat | ...
	MainHand           Hand
	TextFiltering      bool
	AllowServerListing bool // show the bot in the server list player sample
	Particles          ParticleStatus
}

// Default view distance. Vanilla servers assume 2 for a client that never
// sent its settings, so this keeps the bot as light as before; raise it to
// load-test chunk sending like real players (they usually use 8-12).
const DefaultViewDistance = 2

// DefaultClientSettings returns the settings a bot uses unless changed.
func DefaultClientSettings() ClientSettings {
	return ClientSettings{
		Locale:             "en_us",
		ViewDistance:       DefaultViewDistance,
		ChatMode:           ChatFull,
		ChatColors:         true,
		SkinParts:          SkinAll,
		MainHand:           HandRight,
		AllowServerListing: true,
		Particles:          ParticlesAll,
	}
}

func (s ClientSettings) validate() error {
	if s.ViewDistance < 2 || s.ViewDistance > 32 {
		return fmt.Errorf("view distance %d out of range 2..32", s.ViewDistance)
	}
	if len(s.Locale) == 0 || len(s.Locale) > 16 {
		return fmt.Errorf("locale %q must be 1..16 characters", s.Locale)
	}
	return nil
}

// ClientSettings returns the bot's current client settings.
func (b *Bot) ClientSettings() ClientSettings {
	b.settingsMu.Lock()
	defer b.settingsMu.Unlock()
	return b.settings
}

// SetClientSettings changes the client settings. Before Connect they are
// sent during login; while connected they are sent right away.
func (b *Bot) SetClientSettings(s ClientSettings) error {
	if err := s.validate(); err != nil {
		return err
	}
	b.settingsMu.Lock()
	b.settings = s
	b.settingsMu.Unlock()

	if b.conn == nil || b.phase.Load() != phasePlay {
		return nil // sent at the start of the next configuration phase
	}
	return b.writePacket(b.clientInformationPacket(b.version.IDs.SB_ClientInformation))
}

// SetViewDistance changes only the view distance (2..32 chunks).
func (b *Bot) SetViewDistance(chunks int) error {
	s := b.ClientSettings()
	s.ViewDistance = chunks
	return b.SetClientSettings(s)
}

// sendClientInformation sends the settings in the configuration phase.
func (b *Bot) sendClientInformation() error {
	return b.writeRaw(b.clientInformationPacket(b.version.IDs.SB_ClientInformation_Config))
}

func (b *Bot) clientInformationPacket(id int32) pk.Packet {
	s := b.ClientSettings()
	return pk.Marshal(
		pk.VarInt(id),
		pk.String(s.Locale),
		pk.Byte(s.ViewDistance),
		pk.VarInt(s.ChatMode),
		pk.Boolean(s.ChatColors),
		pk.UnsignedByte(s.SkinParts),
		pk.VarInt(s.MainHand),
		pk.Boolean(s.TextFiltering),
		pk.Boolean(s.AllowServerListing),
		pk.VarInt(s.Particles),
	)
}
