package main

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func configString(config, key string) string {
	for _, line := range strings.Split(config, "\n") {
		if strings.HasPrefix(line, key+" = ") {
			v, _ := strconv.Unquote(strings.TrimPrefix(line, key+" = "))
			return v
		}
	}
	return ""
}
func TestSecureModeContainsActualX25519Keypair(t *testing.T) {
	r := testRoom(t)
	cfg, err := engineConfig(r, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", false)
	if err != nil {
		t.Fatal(err)
	}
	private, err := base64.StdEncoding.DecodeString(configString(cfg, "local_private_key"))
	if err != nil {
		t.Fatal("private key encoding invalid")
	}
	key, err := ecdh.X25519().NewPrivateKey(private)
	if err != nil {
		t.Fatal("invalid private key")
	}
	if configString(cfg, "local_public_key") != base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()) {
		t.Fatal("public key not derived from actual private key")
	}
	next, err := engineConfig(r, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", false)
	if err != nil {
		t.Fatal(err)
	}
	if configString(next, "local_public_key") == configString(cfg, "local_public_key") {
		t.Fatal("reused ephemeral keypair")
	}
}
func TestOfficialSecurePeerPresenceAndPin(t *testing.T) {
	engineDir := os.Getenv("ELCIBER_TEST_ENGINE_DIR")
	if engineDir == "" {
		t.Skip("set ELCIBER_TEST_ENGINE_DIR for the real loopback-only two-peer test")
	}
	dir := privateTestDir(t)
	e, err := NewEngine(engineDir, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	room := testRoom(t)
	foreignRoom := testRoom(t)
	inst, err := newInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	referenceConfig, err := engineConfig(room, inst, false)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	peerAddr := l.Addr().String()
	_ = l.Close()
	l, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rpcAddr := l.Addr().String()
	_ = l.Close()
	referenceConfig = strings.Replace(referenceConfig, `hostname = "ElCiber"`, `hostname = "Peer-de-prueba"`, 1)
	referenceConfig = strings.Replace(referenceConfig, "listeners = []", "listeners = ["+strconv.Quote("tcp://"+peerAddr)+"]", 1)
	// Only this loopback reference accepts foreign networks; it still has no
	// TUN, STUN, routes, external connector or relay-data capability.
	referenceConfig = strings.Replace(referenceConfig, "private_mode = true", "private_mode = false", 1)
	// Register exactly this synthetic foreign network. Data forwarding remains
	// disabled and the listener is numeric loopback; production clients keep
	// their relay whitelist empty.
	referenceConfig = strings.Replace(referenceConfig, `relay_network_whitelist = ""`, "relay_network_whitelist = "+strconv.Quote("elciber-"+foreignRoom.ID), 1)
	referenceDir := privateTestDir(t)
	cfgPath := filepath.Join(referenceDir, "reference.toml")
	if err := os.WriteFile(cfgPath, []byte(referenceConfig), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(e.core, "--config-file", cfgPath, "--disable-env-parsing", "--rpc-portal", rpcAddr, "--rpc-portal-whitelist", "127.0.0.1/32", "--console-log-level", "off", "--file-log-level", "off")
	cmd.Dir = referenceDir
	cmd.Env = privateEnv(referenceDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal("reference start failed")
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	defer func() {
		_ = terminateProcess(cmd.Process, false)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = terminateProcess(cmd.Process, true)
			<-done
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	ref := &engineProcess{rpc: rpcAddr, dir: referenceDir, instanceID: inst}
	for {
		if _, err := e.readNode(ctx, ref); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("reference RPC unavailable")
		case <-done:
			t.Fatal("reference exited")
		case <-time.After(100 * time.Millisecond):
		}
	}
	room.Rendezvous = "tcp://" + peerAddr
	// This is an explicit regression for a documented upstream limitation:
	// knowing the room secret takes precedence over a mismatched peer pin.
	sameSecretDifferentKey, err := engineConfig(testRoom(t), inst, false)
	if err != nil {
		t.Fatal(err)
	}
	room.RendezvousKey = configString(sameSecretDifferentKey, "local_public_key")
	if err := e.Connect(ctx, room); err != nil {
		t.Fatal(err)
	}
	for {
		state := e.Snapshot()
		found := false
		for _, p := range state.Peers {
			if p.Hostname == "Peer-de-prueba" && p.Connection == "direct" {
				found = true
			}
		}
		if found {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("real secure peer was not observed")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if e.Snapshot().VirtualIP != "" {
		t.Fatal("inspection acquired an IP")
	}
	e.Disconnect()
	// Upstream gives a valid same-network secret proof precedence over a pin.
	// Test the pin's actual boundary: a rendezvous outside the client's room.
	room = foreignRoom
	room.Rendezvous = "tcp://" + peerAddr
	room.RendezvousKey = configString(referenceConfig, "local_public_key")
	connected := func() bool {
		e.mu.RLock()
		p := e.proc
		e.mu.RUnlock()
		if p == nil {
			return false
		}
		b, err := runPrivate(ctx, e.cli, p.dir, []string{"-p", p.rpc, "-o", "json", "connector"})
		if err != nil {
			return false
		}
		var rows []struct {
			Status *int `json:"status"`
		}
		if json.Unmarshal(b, &rows) != nil {
			return false
		}
		return len(rows) == 1 && rows[0].Status != nil && *rows[0].Status == 0
	}
	if err := e.Connect(ctx, room); err != nil {
		t.Fatal(err)
	}
	for !connected() {
		select {
		case <-ctx.Done():
			t.Fatal("correct shared-node pin did not connect")
		case <-time.After(100 * time.Millisecond):
		}
	}
	e.Disconnect()
	// Pin a DIFFERENT freshly generated real public key. A healthy RPC alone
	// must not be confused with a successfully authenticated connector.
	otherConfig, err := engineConfig(testRoom(t), inst, false)
	if err != nil {
		t.Fatal(err)
	}
	room.RendezvousKey = configString(otherConfig, "local_public_key")
	if err := e.Connect(ctx, room); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			e.Disconnect()
			t.Log("actual Noise/X25519 peer presence passed; foreign rendezvous connected with correct pin and rejected a wrong pin; no TUN or external destination")
			return
		case <-ctx.Done():
			t.Fatal("pin regression deadline")
		case <-time.After(100 * time.Millisecond):
			if connected() {
				t.Fatal("incorrect public key pin accepted")
			}
		}
	}
}
