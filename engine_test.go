package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// A real subprocess, but no network: unit lifecycle tests deliberately simulate
// the RPC seam. The separate opt-in integration test uses official EasyTier.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == "--config-file" {
		if _, err := os.Stat(os.Args[2]); err != nil {
			os.Exit(80)
		}
		for _, entry := range os.Environ() {
			if strings.HasPrefix(entry, "ET_") || strings.HasPrefix(entry, "AWS_") || strings.HasPrefix(entry, "LD_PRELOAD=") {
				os.Exit(81)
			}
		}
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func helperEngine(t *testing.T, ready bool) *Engine {
	t.Helper()
	dir := privateTestDir(t)
	e, err := NewEngine(privateTestDir(t), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	e.core = exe
	e.cli = exe
	e.readyTimeout = 400 * time.Millisecond
	e.pollInterval = 20 * time.Millisecond
	// Synthetic unit-test version fixture; real integration is tested separately.
	e.run = func(ctx context.Context, bin, dir string, args []string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" {
			return []byte("easytier-core 2.6.4-abcdef0"), nil
		}
		if !ready {
			return nil, errEngine
		}
		id := ""
		for i, arg := range args {
			if arg == "-i" && i+1 < len(args) {
				id = args[i+1]
			}
		}
		if args[len(args)-2] == "node" {
			return json.Marshal(nodeInfo{PeerID: 123, InstanceID: id, Version: "2.6.4-abcdef0"})
		}
		return []byte(`[{"id":"123","hostname":"local","cost":"Local","lat_ms":"-"}]`), nil
	}
	t.Cleanup(e.Close)
	return e
}
func TestPrivateEnvironment(t *testing.T) {
	for _, key := range []string{"ET_CONFIG_SERVER", "ET_PEERS", "ET_IPV4", "ET_ACCEPT_DNS", "ET_CONFIG_FILE", "AWS_SECRET_ACCESS_KEY", "HTTPS_PROXY", "LD_PRELOAD", "RUST_LOG"} {
		t.Setenv(key, "must-not-inherit")
	}
	env := privateEnv(privateTestDir(t))
	for _, s := range env {
		if strings.Contains(s, "must-not-inherit") || strings.HasPrefix(s, "ET_") {
			t.Fatal("inherited unsafe env")
		}
	}
}
func TestEngineConfigSecurity(t *testing.T) {
	r := testRoom(t)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	config, err := engineConfig(r, id, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"dhcp = false", "no_tun = true", "stun_servers = []", "stun_servers_v6 = []", "listeners = []", "routes = []", "exit_nodes = []", "proxy_network = []", "port_forward = []", "enabled = true", "enable_encryption = true", "encryption_algorithm = \"chacha20\"", "disable_p2p = true", "disable_udp_hole_punching = true", "disable_tcp_hole_punching = true", "disable_upnp = true", "accept_dns = false", "relay_network_whitelist = \"\"", "enable_exit_node = false", "enable_udp_broadcast_relay = false", "disable_kcp_input = true", "disable_quic_input = true"} {
		if !strings.Contains(config, s) {
			t.Error("missing invariant", s)
		}
	}
	if strings.Contains(config, "[[peer]]") {
		t.Fatal("automatic default peer")
	}
	r.Rendezvous = "tcp://example.com:11010"
	if _, err := engineConfig(r, id, false); err == nil {
		t.Fatal("inspection external endpoint")
	}
	if _, err := engineConfig(r, id, true); err == nil {
		t.Fatal("VPN accepted missing pin")
	}
	// This is a public-key shape fixture only; no claim it pins a real node.
	r.RendezvousKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	vpn, err := engineConfig(r, id, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"dhcp = true", "no_tun = false", "peer_public_key = \"" + r.RendezvousKey + "\"", "routes = []", "[secure_mode]\nenabled = true"} {
		if !strings.Contains(vpn, s) {
			t.Fatal("VPN invariant missing", s)
		}
	}
	if strings.Contains(vpn, "10.144.") || strings.Contains(vpn, "dhcp_range") {
		t.Fatal("invented DHCP range")
	}
}
func TestLifecycleRequiresRPCAndCleansFailure(t *testing.T) {
	e := helperEngine(t, false)
	r := testRoom(t)
	if err := e.Connect(context.Background(), r); !errors.Is(err, errEngine) {
		t.Fatal("start without RPC accepted", err)
	}
	s := e.Snapshot()
	if s.Status != "error" || e.ActiveRoom() != "" {
		t.Fatal("stale active process", s)
	}
	dirs, _ := filepath.Glob(filepath.Join(e.dataDir, ".engine-*"))
	if len(dirs) != 0 {
		t.Fatal("private config leaked")
	}
}
func TestLifecycleReadyDisconnectCrashAndSecretIsolation(t *testing.T) {
	e := helperEngine(t, true)
	r := testRoom(t)
	if err := e.Connect(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if e.Snapshot().Status != "running" {
		t.Fatal("not ready")
	}
	if err := e.Connect(context.Background(), r); !errors.Is(err, errActive) {
		t.Fatal("duplicate process")
	}
	e.mu.RLock()
	p := e.proc
	e.mu.RUnlock()
	if strings.Contains(strings.Join(p.cmd.Args, " "), r.Secret) {
		t.Fatal("secret in argv")
	}
	config, err := os.ReadFile(filepath.Join(p.dir, "network.toml"))
	if err != nil || !strings.Contains(string(config), r.Secret) {
		t.Fatal("private configuration missing")
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(filepath.Join(p.dir, "network.toml"))
		if st.Mode().Perm() != 0600 {
			t.Fatal("unsafe permissions")
		}
	}
	e.Disconnect()
	e.Disconnect()
	if e.Snapshot().Status != "stopped" || e.ActiveRoom() != "" {
		t.Fatal("disconnect failed")
	}
	select {
	case <-p.done:
	default:
		t.Fatal("child not reaped")
	}
	if _, err := os.Stat(p.dir); !os.IsNotExist(err) {
		t.Fatal("config not removed")
	}
	if err := e.Connect(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	e.mu.RLock()
	p = e.proc
	e.mu.RUnlock()
	_ = terminateProcess(p.cmd.Process, true)
	deadline := time.After(2 * time.Second)
	for e.Snapshot().Status != "error" {
		select {
		case <-deadline:
			t.Fatal("crash not observed")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if e.ActiveRoom() != "" {
		t.Fatal("crashed room remains active")
	}
}
func TestLifecycleCloseAndCanceledRequest(t *testing.T) {
	e := helperEngine(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e.Connect(ctx, testRoom(t)) == nil {
		t.Fatal("canceled connect started")
	}
	e = helperEngine(t, true)
	r := testRoom(t)
	if err := e.Connect(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	e.mu.RLock()
	p := e.proc
	e.mu.RUnlock()
	e.Close()
	select {
	case <-p.done:
	default:
		t.Fatal("shutdown orphaned child")
	}
	if e.Connect(context.Background(), r) == nil {
		t.Fatal("closed engine restarted")
	}
}
func TestEngineMissingUnsupportedVersionAndRPCIdentity(t *testing.T) {
	e := helperEngine(t, true)
	r := testRoom(t)
	e.run = func(context.Context, string, string, []string) ([]byte, error) {
		return []byte("easytier-core 2.6.3"), nil
	}
	if !errors.Is(e.Connect(context.Background(), r), errEngineVersion) {
		t.Fatal("unsupported engine launched")
	}
	e.core = filepath.Join(privateTestDir(t), "missing")
	if !errors.Is(e.Connect(context.Background(), r), errEngineMissing) {
		t.Fatal("missing engine hidden")
	}
	e = helperEngine(t, true)
	e.run = func(ctx context.Context, bin, dir string, args []string) ([]byte, error) {
		if len(args) == 1 {
			return []byte("easytier-core 2.6.4"), nil
		}
		return []byte(`{"peer_id":1,"inst_id":"another-instance","version":"2.6.4"}`), nil
	}
	if e.Connect(context.Background(), r) == nil {
		t.Fatal("unrelated RPC satisfies readiness")
	}
}
func TestLifecycleRPCFailureClearsEvidence(t *testing.T) {
	e := helperEngine(t, true)
	base := e.run
	var fail atomic.Bool
	e.run = func(ctx context.Context, bin, dir string, args []string) ([]byte, error) {
		if fail.Load() {
			return nil, errEngine
		}
		return base(ctx, bin, dir, args)
	}
	if err := e.Connect(context.Background(), testRoom(t)); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	deadline := time.After(time.Second)
	for e.Snapshot().Status != "error" {
		select {
		case <-deadline:
			t.Fatal("RPC failure invisible")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	s := e.Snapshot()
	if len(s.Peers) != 0 || s.VirtualIP != "" {
		t.Fatal("stale evidence displayed")
	}
}
func TestParseRealPeerSchema(t *testing.T) {
	b := []byte(`[{"id":"1","cost":"Local","hostname":"local","lat_ms":"-"},{"id":"2","cost":"p2p","hostname":"amigo","ipv4":"10.126.126.2","lat_ms":"1.25","tunnel_proto":"tcp"},{"id":"3","cost":"relay(2)","hostname":"otra","ipv4":"invalid","lat_ms":"NaN","tunnel_proto":""},{"id":"4","cost":"unknown","hostname":"bad\nname","lat_ms":"0","tunnel_proto":""}]`)
	peers, err := parsePeers(b, 1)
	if err != nil || len(peers) != 3 {
		t.Fatal("parse failed", err)
	}
	if peers[0].Connection != "direct" || peers[0].LatencyMS == nil || *peers[0].LatencyMS != 1.25 || peers[1].Connection != "relay" || peers[1].IPv4 != "" || peers[1].LatencyMS != nil || peers[2].Hostname != "" || peers[2].LatencyMS != nil {
		t.Fatal("invalid/ fabricated evidence", peers)
	}
	for _, b := range []string{`null`, `{}`, `[{"id":"oops"}]`} {
		if _, err := parsePeers([]byte(b), 1); err == nil {
			t.Fatal("invalid schema accepted")
		}
	}
}
func TestConcurrentEngineSnapshots(t *testing.T) {
	e := helperEngine(t, true)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				_ = e.Snapshot()
				_ = e.ActiveRoom()
			}
		})
	}
	if err := e.Connect(context.Background(), testRoom(t)); err != nil {
		t.Fatal(err)
	}
	e.Disconnect()
	wg.Wait()
}
func TestActiveRoomCannotBeDeleted(t *testing.T) {
	a := testAPI(t)
	e := helperEngine(t, true)
	a.engine = e
	r := testRoom(t)
	_, _ = a.store.Add(r)
	if err := e.Connect(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	w := request(a, "DELETE", "/api/rooms/"+r.ID, `{}`)
	if w.Code != 409 {
		t.Fatal("deleted active room", w.Code)
	}
}
func TestOfficialEasyTierInspection(t *testing.T) {
	dir := os.Getenv("ELCIBER_TEST_ENGINE_DIR")
	if dir == "" {
		t.Skip("set ELCIBER_TEST_ENGINE_DIR to run official loopback/no-tun integration")
	}
	e, err := NewEngine(dir, privateTestDir(t), false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	r := testRoom(t)
	// Hostile parent configuration must not affect the real core or CLI.
	for _, key := range []string{"ET_CONFIG_SERVER", "ET_PEERS", "ET_IPV4", "ET_ACCEPT_DNS", "ET_STUN_SERVERS", "ET_NETWORK_SECRET", "ET_ENABLE_UDP_BROADCAST_RELAY"} {
		t.Setenv(key, "must-not-be-inherited")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.Connect(ctx, r); err != nil {
		t.Fatal(err)
	}
	s := e.Snapshot()
	if s.Status != "running" || s.Mode != "inspection" || s.VirtualIP != "" || !versionPattern.MatchString(s.Version) {
		t.Fatal("unexpected real node state", s.Status, s.Version)
	}
	e.mu.RLock()
	p := e.proc
	e.mu.RUnlock()
	peers, err := e.readPeers(ctx, p, 0)
	if err != nil || len(peers) != 0 {
		t.Fatal("unexpected peers", err, len(peers))
	}
	// Check both configurations with the official parser without running VPN.
	r.Rendezvous = "tcp://127.0.0.1:11010"
	r.RendezvousKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	for _, vpn := range []bool{false, true} {
		conf, err := engineConfig(r, p.instanceID, vpn)
		if err != nil {
			t.Fatal(err)
		}
		cfg := filepath.Join(p.dir, fmt.Sprintf("check-%t.toml", vpn))
		if err := os.WriteFile(cfg, []byte(conf), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := runPrivate(ctx, e.core, p.dir, []string{"--config-file", cfg, "--disable-env-parsing", "--check-config"}); err != nil {
			t.Fatal("official parser rejects generated config", vpn)
		}
		_ = os.Remove(cfg)
	}
	e.Disconnect()
	select {
	case <-p.done:
	default:
		t.Fatal("real child not reaped")
	}
	if _, err := os.Stat(p.dir); !os.IsNotExist(err) {
		t.Fatal("real config leaked")
	}
	t.Log("official EasyTier 2.6.4: readiness via node JSON, peers JSON, isolated env, no-tun, parser checks inspection/VPN, shutdown cleanup passed")
}
