package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEngineConfigSeparatesInspectionAndVPNConnectivity(t *testing.T) {
	r := testRoom(t)
	instanceID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"

	inspection, err := engineConfig(r, instanceID, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{
		"no_tun = true",
		"stun_servers = []",
		"stun_servers_v6 = []",
		"disable_p2p = true",
		"disable_tcp_hole_punching = true",
		"disable_udp_hole_punching = true",
		"disable_sym_hole_punching = true",
		"disable_relay_data = true",
		"disable_upnp = true",
	} {
		if !strings.Contains(inspection, invariant) {
			t.Fatalf("inspection lost isolation invariant %q", invariant)
		}
	}

	r.Rendezvous = "tcp://example.com:11010"
	r.RendezvousKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	vpn, err := engineConfig(r, instanceID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{
		"no_tun = false",
		"disable_p2p = false",
		"disable_tcp_hole_punching = false",
		"disable_udp_hole_punching = false",
		"disable_sym_hole_punching = false",
		"disable_relay_data = false",
		"disable_upnp = true",
		"disable_relay_kcp = true",
		"disable_relay_quic = true",
		"private_mode = true",
		"relay_network_whitelist = \"\"",
	} {
		if !strings.Contains(vpn, invariant) {
			t.Fatalf("VPN connectivity invariant missing %q", invariant)
		}
	}
	for _, isolated := range []string{"stun_servers = []", "stun_servers_v6 = []"} {
		if strings.Contains(vpn, isolated) {
			t.Fatalf("VPN still disables EasyTier discovery with %q", isolated)
		}
	}
}
