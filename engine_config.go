package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const supportedEngineVersion = "2.6.4"

// v2.6.4 DHCP uses 10.126.126.0/24 or adopts a subnet from peers.
// There is no configurable DHCP range. routes=[] explicitly disables learned
// subnet/portal routes, not the connected route for the virtual adapter.

func engineConfig(r savedRoom, instanceID string, vpn bool) (string, error) {
	if validateRoom(r) != nil {
		return "", errInvalid
	}
	if vpn {
		if r.Rendezvous == "" {
			return "", errors.New("VPN requiere un nodo de encuentro explícito.")
		}
		if r.RendezvousKey == "" {
			return "", errors.New("VPN requiere la clave pública del nodo de encuentro.")
		}
	}
	if !vpn && validateEndpoint(r.Rendezvous, true) != nil {
		return "", errors.New("En inspección solo se permiten destinos loopback numéricos.")
	}
	// Loading TOML does NOT call EasyTier's CLI process_secure_mode_cfg.
	// enabled=true alone passes --check-config but fails every Noise handshake.
	// Generate an actual X25519 keypair; only the private TOML receives it.
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", errEngine
	}
	// All interpolated values have restrictive validation; quoted TOML strings
	// cannot carry user-defined keys, routes, environment references or commands.
	var b strings.Builder
	fmt.Fprintf(&b, "instance_name = \"elciber\"\ninstance_id = %q\nhostname = \"ElCiber\"\n", instanceID)
	fmt.Fprintf(&b, "dhcp = %t\n", vpn)
	b.WriteString(`
ipv6_public_addr_provider = false
ipv6_public_addr_auto = false
listeners = []
mapped_listeners = []
exit_nodes = []
routes = []
proxy_network = []
port_forward = []
stun_servers = []
stun_servers_v6 = []
`)
	fmt.Fprintf(&b, "[network_identity]\nnetwork_name = %q\nnetwork_secret = %q\n", "elciber-"+r.ID, r.Secret)
	fmt.Fprintf(&b, "[secure_mode]\nenabled = true\nlocal_private_key = %q\nlocal_public_key = %q\n", base64.StdEncoding.EncodeToString(key.Bytes()), base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()))
	b.WriteString(`
[flags]
no_tun = true
enable_encryption = true
encryption_algorithm = "chacha20"
enable_ipv6 = false
disable_p2p = true
disable_tcp_hole_punching = true
disable_udp_hole_punching = true
disable_sym_hole_punching = true
disable_upnp = true
accept_dns = false
enable_exit_node = false
proxy_forward_by_system = false
use_smoltcp = false
relay_network_whitelist = ""
relay_all_peer_rpc = false
need_p2p = false
private_mode = true
enable_kcp_proxy = false
disable_kcp_input = true
enable_quic_proxy = false
disable_quic_input = true
disable_relay_data = true
disable_relay_kcp = true
disable_relay_quic = true
enable_relay_foreign_network_kcp = false
enable_relay_foreign_network_quic = false
enable_udp_broadcast_relay = false
bind_device = false
multi_thread = false
`)
	if r.Rendezvous != "" {
		fmt.Fprintf(&b, "[[peer]]\nuri = %q\n", r.Rendezvous)
		if r.RendezvousKey != "" {
			fmt.Fprintf(&b, "peer_public_key = %q\n", r.RendezvousKey)
		}
	}
	config := b.String()
	if vpn {
		config = strings.Replace(config, "no_tun = true", "no_tun = false", 1)
	}
	return config, nil
}
