// Command genpackets generates a version's packet ID table (types.PacketIDs)
// from the vanilla data generator's reports/packets.json.
//
//	java -DbundlerMainClass=net.minecraft.data.Main -jar server.jar --reports --output gen
//	go run ./internal/tools/genpackets -report gen/reports/packets.json \
//	    -version 26.2 -out internal/protocol/versions/v776/packets.go -pkg v776
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
)

// fields maps each types.PacketIDs field to Mojang's packet name.
var fields = []struct{ field, state, direction, name string }{
	{"SB_Handshake", "handshake", "serverbound", "minecraft:intention"},
	{"SB_LoginStart", "login", "serverbound", "minecraft:hello"},
	{"SB_LoginAck", "login", "serverbound", "minecraft:login_acknowledged"},
	{"CB_LoginSuccess", "login", "clientbound", "minecraft:login_finished"},
	{"CB_Disconnect", "login", "clientbound", "minecraft:login_disconnect"},
	{"CB_SetCompression", "login", "clientbound", "minecraft:login_compression"},
	{"SB_KnownPacks", "configuration", "serverbound", "minecraft:select_known_packs"},
	{"SB_FinishConfig", "configuration", "serverbound", "minecraft:finish_configuration"},
	{"SB_PluginResponse", "configuration", "serverbound", "minecraft:custom_payload"},
	{"CB_KnownPacks", "configuration", "clientbound", "minecraft:select_known_packs"},
	{"CB_RegistryData", "configuration", "clientbound", "minecraft:registry_data"},
	{"CB_FinishConfig", "configuration", "clientbound", "minecraft:finish_configuration"},
	{"CB_PluginRequest", "configuration", "clientbound", "minecraft:custom_payload"},
	{"CB_Disconnect_Config", "configuration", "clientbound", "minecraft:disconnect"},
	{"CB_KeepAlive_Config", "configuration", "clientbound", "minecraft:keep_alive"},
	{"SB_KeepAlive_Config", "configuration", "serverbound", "minecraft:keep_alive"},
	{"CB_FeatureFlags", "configuration", "clientbound", "minecraft:update_enabled_features"},
	{"SB_AcceptTeleport", "play", "serverbound", "minecraft:accept_teleportation"},
	{"SB_ChatCommand", "play", "serverbound", "minecraft:chat_command"},
	{"SB_Chat", "play", "serverbound", "minecraft:chat"},
	{"SB_ClientCommand", "play", "serverbound", "minecraft:client_command"},
	{"SB_ClientTickEnd", "play", "serverbound", "minecraft:client_tick_end"},
	{"SB_ClientInformation", "play", "serverbound", "minecraft:client_information"},
	{"SB_KeepAlive", "play", "serverbound", "minecraft:keep_alive"},
	{"SB_PlayerPosition", "play", "serverbound", "minecraft:move_player_pos"},
	{"SB_PlayerPositionRotation", "play", "serverbound", "minecraft:move_player_pos_rot"},
	{"SB_PlayerRotation", "play", "serverbound", "minecraft:move_player_rot"},
	{"SB_PlayerOnGround", "play", "serverbound", "minecraft:move_player_status_only"},
	{"SB_PlayerCommand", "play", "serverbound", "minecraft:player_command"},
	{"SB_PlayerAction", "play", "serverbound", "minecraft:player_action"},
	{"SB_PlayerInput", "play", "serverbound", "minecraft:player_input"},
	{"SB_ChunkBatchReceived", "play", "serverbound", "minecraft:chunk_batch_received"},
	{"SB_PlayerLoaded", "play", "serverbound", "minecraft:player_loaded"},
	{"SB_UseItem", "play", "serverbound", "minecraft:use_item"},
	{"CB_ChunkBatchFinished", "play", "clientbound", "minecraft:chunk_batch_finished"},
	{"CB_ChunkBatchStart", "play", "clientbound", "minecraft:chunk_batch_start"},
	{"CB_SpawnEntity", "play", "clientbound", "minecraft:add_entity"},
	{"CB_SectionBlocksUpdate", "play", "clientbound", "minecraft:section_blocks_update"},
	{"CB_BlockUpdate", "play", "clientbound", "minecraft:block_update"},
	{"CB_Disconnect_Play", "play", "clientbound", "minecraft:disconnect"},
	{"CB_UnloadChunk", "play", "clientbound", "minecraft:forget_level_chunk"},
	{"CB_GameEvent", "play", "clientbound", "minecraft:game_event"},
	{"CB_KeepAlive", "play", "clientbound", "minecraft:keep_alive"},
	{"CB_ChunkData", "play", "clientbound", "minecraft:level_chunk_with_light"},
	{"CB_Login", "play", "clientbound", "minecraft:login"},
	{"CB_PlayerChat", "play", "clientbound", "minecraft:player_chat"},
	{"CB_CombatDeath", "play", "clientbound", "minecraft:player_combat_kill"},
	{"CB_SyncPosition", "play", "clientbound", "minecraft:player_position"},
	{"CB_Respawn", "play", "clientbound", "minecraft:respawn"},
	{"CB_SetDefaultSpawnPosition", "play", "clientbound", "minecraft:set_default_spawn_position"},
	{"CB_UpdateHealth", "play", "clientbound", "minecraft:set_health"},
	{"CB_SystemChat", "play", "clientbound", "minecraft:system_chat"},
}

type report map[string]map[string]map[string]struct {
	ProtocolID int `json:"protocol_id"`
}

func main() {
	path := flag.String("report", "", "reports/packets.json from the vanilla data generator")
	version := flag.String("version", "", "Minecraft version (for the header)")
	out := flag.String("out", "", "output Go file")
	pkg := flag.String("pkg", "", "output package name")
	flag.Parse()
	if *path == "" || *version == "" || *out == "" || *pkg == "" {
		flag.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		log.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		log.Fatalf("%s: %v", *path, err)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by internal/tools/genpackets from Mojang reports/packets.json (%s). DO NOT EDIT.\n\n", *version)
	fmt.Fprintf(&buf, "package %s\n\n", *pkg)
	fmt.Fprintf(&buf, "import \"github.com/deware-pk/go-mcbots/internal/protocol/types\"\n\n")
	fmt.Fprintf(&buf, "// IDs are the packet IDs of Minecraft %s.\n", *version)
	fmt.Fprintf(&buf, "var IDs = types.PacketIDs{\n")
	for _, f := range fields {
		p, ok := r[f.state][f.direction][f.name]
		if !ok {
			log.Fatalf("%s: packet %s/%s/%s not in report", f.field, f.state, f.direction, f.name)
		}
		fmt.Fprintf(&buf, "\t%s: 0x%02X, // %s %s %s\n", f.field, p.ProtocolID, f.state, f.direction, f.name)
	}
	fmt.Fprintf(&buf, "}\n")

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("format: %v", err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d packet IDs to %s", len(fields), *out)
}
