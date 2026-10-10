package bot

import (
	"net"
	"testing"
	"time"

	mcnet "github.com/deware-pk/go-mcbots/internal/protocol/net"
	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

// fakeServer drives one bot connection through login, configuration and play.
type fakeServer struct {
	t    *testing.T
	conn *mcnet.Conn
	ids  Version
}

func (s *fakeServer) read() pk.Packet {
	s.t.Helper()
	var p pk.Packet
	if err := s.conn.ReadPacket(&p); err != nil {
		s.t.Fatalf("server read: %v", err)
	}
	return p
}

// readUntil reads packets until one with ID id arrives and returns the IDs
// read before it.
func (s *fakeServer) readUntil(id int32) (pk.Packet, []int32) {
	s.t.Helper()
	var before []int32
	for {
		p := s.read()
		if p.ID == id {
			return p, before
		}
		before = append(before, p.ID)
	}
}

func (s *fakeServer) write(p pk.Packet) {
	s.t.Helper()
	if err := s.conn.WritePacket(p); err != nil {
		s.t.Fatalf("server write: %v", err)
	}
}

// configure runs a configuration phase from the server side and checks the
// client only sends configuration packets.
func (s *fakeServer) configure() (viewDistance int8) {
	s.t.Helper()
	ids := s.ids.IDs
	info := s.read()
	if info.ID != ids.SB_ClientInformation_Config {
		s.t.Fatalf("first configuration packet 0x%02X, want client information", info.ID)
	}
	var locale pk.String
	var vd pk.Byte
	if err := info.Scan(&locale, &vd); err != nil {
		s.t.Fatal(err)
	}

	// A ping and a resource pack must be answered.
	s.write(pk.Marshal(ids.CB_Ping_Config, pk.Int(77)))
	s.write(pk.Marshal(ids.CB_ResourcePackPush_Config, pk.UUID{1}, pk.String("http://x/p.zip"), pk.String(""), pk.Boolean(true), pk.Boolean(false)))
	s.write(pk.Marshal(ids.CB_FinishConfig))

	allowed := map[int32]bool{ids.SB_PluginResponse: true, ids.SB_Pong_Config: true, ids.SB_ResourcePack_Config: true}
	_, before := s.readUntil(ids.SB_FinishConfig)
	var pongs, packs int
	for _, id := range before {
		if !allowed[id] {
			s.t.Fatalf("non-configuration packet 0x%02X during configuration", id)
		}
		if id == ids.SB_Pong_Config {
			pongs++
		}
		if id == ids.SB_ResourcePack_Config {
			packs++
		}
	}
	if pongs != 1 || packs != 3 {
		s.t.Fatalf("got %d pongs and %d resource pack responses, want 1 and 3", pongs, packs)
	}
	return int8(vd)
}

func (s *fakeServer) joinWorld() {
	ids := s.ids.IDs
	s.write(pk.Marshal(ids.CB_Login,
		pk.Int(7), pk.Boolean(false), pk.VarInt(1), pk.String("minecraft:overworld"),
		pk.VarInt(20), pk.VarInt(10), pk.VarInt(10),
		pk.Boolean(false), pk.Boolean(true), pk.Boolean(false),
		pk.VarInt(0), pk.String("minecraft:overworld"),
	))
	s.write(pk.Marshal(ids.CB_SyncPosition,
		pk.VarInt(1), pk.Double(0.5), pk.Double(64), pk.Double(0.5),
		pk.Double(0), pk.Double(0), pk.Double(0), pk.Float(0), pk.Float(0), pk.Int(0)))
	s.readUntil(ids.SB_AcceptTeleport)
}

func TestProxyServerSwitchReconfigures(t *testing.T) {
	ver, err := ResolveVersion("26.2")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	b := New("Switcher", ver)
	if err := b.SetViewDistance(7); err != nil {
		t.Fatal(err)
	}
	spawns := make(chan struct{}, 4)
	reconfigs := 0
	b.Events.OnSpawn = func() { spawns <- struct{}{} }
	b.Events.OnReconfigure = func() { reconfigs++ }

	done := make(chan error, 1)
	go func() { done <- b.Connect(ln.Addr().String()) }()

	raw, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{t: t, conn: mcnet.WrapConn(raw), ids: ver}
	defer s.conn.Close()
	ids := ver.IDs

	hs := s.read()
	var proto pk.VarInt
	var host pk.String
	var port pk.UnsignedShort
	if err := hs.Scan(&proto, &host, &port); err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" || int(port) != ln.Addr().(*net.TCPAddr).Port {
		t.Errorf("handshake host/port = %q/%d", host, port)
	}
	s.read() // login start
	s.write(pk.Marshal(ids.CB_LoginSuccess, pk.UUID{}, pk.String("Switcher"), pk.VarInt(0)))
	s.readUntil(ids.SB_LoginAck)

	if vd := s.configure(); vd != 7 {
		t.Errorf("view distance sent = %d, want 7", vd)
	}
	s.joinWorld()
	waitSpawn(t, spawns)

	// Let physics send some movement, then switch servers.
	time.Sleep(150 * time.Millisecond)
	s.write(pk.Marshal(ids.CB_StartConfiguration))
	s.readUntil(ids.SB_ConfigurationAck)
	if vd := s.configure(); vd != 7 {
		t.Errorf("view distance after switch = %d, want 7", vd)
	}
	s.joinWorld()
	waitSpawn(t, spawns)

	if reconfigs != 1 {
		t.Errorf("OnReconfigure fired %d times, want 1", reconfigs)
	}
	if err := b.Chat("hello"); err != nil {
		t.Errorf("chat after switch: %v", err)
	}
	p, _ := s.readUntil(ids.SB_Chat)
	var msg pk.String
	p.Scan(&msg)
	if msg != "hello" {
		t.Errorf("chat = %q", msg)
	}

	// SetViewDistance in play sends the play-state packet.
	if err := b.SetViewDistance(12); err != nil {
		t.Fatal(err)
	}
	p, _ = s.readUntil(ids.SB_ClientInformation)
	var locale pk.String
	var vd pk.Byte
	p.Scan(&locale, &vd)
	if vd != 12 {
		t.Errorf("play client information view distance = %d", vd)
	}

	b.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Connect did not return after Close")
	}
}

func waitSpawn(t *testing.T, spawns chan struct{}) {
	t.Helper()
	select {
	case <-spawns:
	case <-time.After(2 * time.Second):
		t.Fatal("no spawn")
	}
}

func TestPlayPacketsDroppedOutsidePlay(t *testing.T) {
	ver, _ := ResolveVersion("26.2")
	b := New("x", ver)
	if err := b.Chat("hi"); err != ErrNotInPlay {
		t.Fatalf("Chat before connect = %v, want ErrNotInPlay", err)
	}
}

func TestClientSettingsValidation(t *testing.T) {
	ver, _ := ResolveVersion("26.2")
	b := New("x", ver)
	if err := b.SetViewDistance(1); err == nil {
		t.Error("view distance 1 accepted")
	}
	if err := b.SetViewDistance(33); err == nil {
		t.Error("view distance 33 accepted")
	}
	if b.ClientSettings().ViewDistance != DefaultViewDistance {
		t.Error("invalid setting changed the settings")
	}
}

func TestHandshakeTarget(t *testing.T) {
	for _, tt := range []struct {
		addr string
		host string
		port uint16
	}{
		{"localhost:25565", "localhost", 25565},
		{"play.example.com:25577", "play.example.com", 25577},
		{"play.example.com", "play.example.com", 25565},
		{"[::1]:25566", "::1", 25566},
	} {
		h, p := handshakeTarget(tt.addr)
		if h != tt.host || p != tt.port {
			t.Errorf("handshakeTarget(%q) = %q, %d", tt.addr, h, p)
		}
	}
}
