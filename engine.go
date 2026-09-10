package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var errEngine = errors.New("EasyTier no pudo iniciar o responder. Comprueba la instalación 2.6.4 y los permisos del adaptador.")
var errEngineMissing = errors.New("No se encontró el motor integrado. Reinstala El Ciber para recuperarlo; tus salas se conservan.")
var errEngineVersion = errors.New("Versión de EasyTier no compatible; se requieren core y cli 2.6.4.")
var errActive = errors.New("Ya hay una conexión activa; desconéctala primero.")
var versionPattern = regexp.MustCompile(`^2\.6\.4(?:-[0-9a-f]{7,40})?$`)

type Peer struct {
	ID         string   `json:"id"`
	Hostname   string   `json:"hostname"`
	IPv4       string   `json:"ipv4"`
	LatencyMS  *float64 `json:"latencyMs"`
	Connection string   `json:"connection"`
}
type EngineState struct {
	Available bool   `json:"available"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	RoomID    string `json:"roomId"`
	Message   string `json:"message"`
	Version   string `json:"version"`
	Peers     []Peer `json:"peers"`
	VirtualIP string `json:"virtualIP"`
}
type engineProcess struct {
	cmd        *exec.Cmd
	done       chan struct{}
	cancel     context.CancelFunc
	dir        string
	rpc        string
	instanceID string
}
type Engine struct {
	op           sync.Mutex
	mu           sync.RWMutex
	core         string
	cli          string
	dataDir      string
	vpn          bool
	state        EngineState
	proc         *engineProcess
	closed       bool
	readyTimeout time.Duration
	pollInterval time.Duration
	// Test seam for bounded RPC/version failures; never configured by API.
	run func(context.Context, string, string, []string) ([]byte, error)
}

func enginePaths(dir string) (string, string, error) {
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", "", errEngineMissing
		}
		dir = filepath.Join(filepath.Dir(exe), "engines", "easytier")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", errEngineMissing
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return filepath.Join(abs, "easytier-core"+suffix), filepath.Join(abs, "easytier-cli"+suffix), nil
}
func executableFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && (runtime.GOOS == "windows" || st.Mode().Perm()&0111 != 0)
}
func NewEngine(dir, dataDir string, vpn bool) (*Engine, error) {
	core, cli, err := enginePaths(dir)
	if err != nil {
		return nil, err
	}
	mode := "inspection"
	if vpn {
		mode = "vpn"
	}
	e := &Engine{core: core, cli: cli, dataDir: dataDir, vpn: vpn, readyTimeout: 12 * time.Second, pollInterval: 2 * time.Second, run: runPrivate}
	e.state = EngineState{Available: executableFile(core) && executableFile(cli), Mode: mode, Status: "stopped", Message: "Desconectado. Guardar salas no inicia la red.", Peers: []Peer{}}
	if !e.state.Available {
		e.state.Message = errEngineMissing.Error()
	}
	return e, nil
}
func (e *Engine) Snapshot() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.state
	s.Peers = append([]Peer{}, s.Peers...)
	// Clone pointer fields as well: callers cannot mutate shared snapshots.
	for i := range s.Peers {
		if s.Peers[i].LatencyMS != nil {
			v := *s.Peers[i].LatencyMS
			s.Peers[i].LatencyMS = &v
		}
	}
	return s
}
func (e *Engine) ActiveRoom() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.proc != nil {
		return e.state.RoomID
	}
	return ""
}
func (e *Engine) fail(message error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Status = "error"
	e.state.RoomID = ""
	e.state.Message = message.Error()
	e.state.Peers = []Peer{}
	e.state.VirtualIP = ""
	return message
}
func newInstanceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", errEngine
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
func (e *Engine) Connect(ctx context.Context, r savedRoom) error {
	e.op.Lock()
	defer e.op.Unlock()
	e.mu.RLock()
	busy := e.proc != nil
	closed := e.closed
	e.mu.RUnlock()
	if busy {
		return errActive
	}
	if closed {
		return errEngine
	}
	instance, err := newInstanceID()
	if err != nil {
		return e.fail(errEngine)
	}
	config, err := engineConfig(r, instance, e.vpn)
	if err != nil {
		return e.fail(err)
	}
	available := executableFile(e.core) && executableFile(e.cli)
	e.mu.Lock()
	e.state.Available = available
	e.mu.Unlock()
	if !available {
		return e.fail(errEngineMissing)
	}
	dir, err := os.MkdirTemp(e.dataDir, ".engine-*")
	if err != nil {
		return e.fail(errDisk)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()
	for _, bin := range []string{e.core, e.cli} {
		vctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		b, err := e.run(vctx, bin, dir, []string{"--version"})
		cancel()
		if err != nil {
			return e.fail(errEngineVersion)
		}
		parts := strings.Fields(string(b))
		if len(parts) != 2 || (parts[0] != "easytier-core" && parts[0] != "easytier-cli" && parts[0] != "easytier") || !versionPattern.MatchString(parts[1]) {
			return e.fail(errEngineVersion)
		}
	}
	confPath := filepath.Join(dir, "network.toml")
	if err := os.WriteFile(confPath, []byte(config), 0600); err != nil {
		return e.fail(errDisk)
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return e.fail(errEngine)
	}
	rpc := l.Addr().String()
	_ = l.Close()
	args := []string{"--config-file", confPath, "--disable-env-parsing", "--rpc-portal", rpc, "--rpc-portal-whitelist", "127.0.0.1/32", "--no-listener", "--console-log-level", "off", "--file-log-level", "off"}
	cmd := exec.Command(e.core, args...)
	cmd.Dir = dir
	cmd.Env = privateEnv(dir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureProcess(cmd)
	e.mu.Lock()
	e.state.Status = "starting"
	e.state.RoomID = r.ID
	e.state.Message = "Esperando al proceso y RPC de EasyTier."
	e.state.Peers = []Peer{}
	e.state.VirtualIP = ""
	e.mu.Unlock()
	if ctx.Err() != nil {
		return e.fail(errEngine)
	}
	if err := cmd.Start(); err != nil {
		return e.fail(errEngine)
	}
	procCtx, cancel := context.WithCancel(context.Background())
	p := &engineProcess{cmd: cmd, done: make(chan struct{}), cancel: cancel, dir: dir, rpc: rpc, instanceID: instance}
	e.mu.Lock()
	e.proc = p
	e.mu.Unlock()
	keep = true
	go func() { _ = cmd.Wait(); close(p.done) }()
	readyCtx, readyCancel := context.WithTimeout(ctx, e.readyTimeout)
	defer readyCancel()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for {
		node, err := e.readNode(readyCtx, p)
		if err == nil {
			select {
			case <-p.done:
				e.stopLocked()
				return e.fail(errEngine)
			default:
			}
			e.mu.Lock()
			e.state.Status = "running"
			e.state.Version = node.Version
			e.state.VirtualIP = cleanIPv4(node.IPv4)
			e.state.Message = "Motor y RPC activos; esto no garantiza una LAN funcional ni amigos conectados."
			e.mu.Unlock()
			go e.monitor(procCtx, p)
			return nil
		}
		select {
		case <-readyCtx.Done():
			e.stopLocked()
			return e.fail(errEngine)
		case <-p.done:
			e.stopLocked()
			return e.fail(errEngine)
		case <-tick.C:
		}
	}
}
func (e *Engine) Disconnect() {
	e.op.Lock()
	defer e.op.Unlock()
	e.stopLocked()
	e.mu.Lock()
	e.state.Status = "stopped"
	e.state.RoomID = ""
	e.state.Message = "Desconectado."
	e.state.Peers = []Peer{}
	e.state.VirtualIP = ""
	e.mu.Unlock()
}
func (e *Engine) Close() {
	e.op.Lock()
	defer e.op.Unlock()
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.stopLocked()
}
func (e *Engine) stopLocked() {
	e.mu.RLock()
	p := e.proc
	e.mu.RUnlock()
	if p == nil {
		return
	}
	p.cancel()
	_ = terminateProcess(p.cmd.Process, false)
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		_ = terminateProcess(p.cmd.Process, true)
		<-p.done
	}
	_ = os.RemoveAll(p.dir)
	e.mu.Lock()
	if e.proc == p {
		e.proc = nil
	}
	e.mu.Unlock()
}
func (e *Engine) monitor(ctx context.Context, p *engineProcess) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			_ = os.RemoveAll(p.dir)
			e.mu.Lock()
			if e.proc == p {
				e.proc = nil
				e.state.Status = "error"
				e.state.RoomID = ""
				e.state.Peers = []Peer{}
				e.state.VirtualIP = ""
				e.state.Message = "EasyTier se ha detenido; no hay conexión activa."
			}
			e.mu.Unlock()
			return
		case <-ticker.C:
			node, err := e.readNode(ctx, p)
			peers := []Peer{}
			if err == nil {
				peers, err = e.readPeers(ctx, p, node.PeerID)
			}
			e.mu.Lock()
			if e.proc == p && ctx.Err() == nil {
				if err != nil {
					e.state.Status = "error"
					e.state.Message = "El RPC del motor no responde; desconecta y revisa EasyTier."
					e.state.Peers = []Peer{}
					e.state.VirtualIP = ""
				} else {
					e.state.Status = "running"
					e.state.Message = "Motor y RPC activos; esto no garantiza una LAN funcional ni amigos conectados."
					e.state.Peers = peers
					e.state.VirtualIP = cleanIPv4(node.IPv4)
					e.state.Version = node.Version
				}
			}
			e.mu.Unlock()
		}
	}
}

type nodeInfo struct {
	PeerID     uint32 `json:"peer_id"`
	IPv4       string `json:"ipv4_addr"`
	InstanceID string `json:"inst_id"`
	Version    string `json:"version"`
}

func (e *Engine) readNode(ctx context.Context, p *engineProcess) (nodeInfo, error) {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	b, err := e.run(c, e.cli, p.dir, []string{"-p", p.rpc, "-o", "json", "-i", p.instanceID, "node", "info"})
	if err != nil {
		return nodeInfo{}, errEngine
	}
	var n nodeInfo
	// node info includes a secret-bearing config field: never log or expose b.
	if json.Unmarshal(b, &n) != nil || n.PeerID == 0 || n.InstanceID != p.instanceID || !versionPattern.MatchString(n.Version) {
		return nodeInfo{}, errEngine
	}
	return n, nil
}
func (e *Engine) readPeers(ctx context.Context, p *engineProcess, local uint32) ([]Peer, error) {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	b, err := e.run(c, e.cli, p.dir, []string{"-p", p.rpc, "-o", "json", "-i", p.instanceID, "peer", "list"})
	if err != nil {
		return nil, errEngine
	}
	return parsePeers(b, local)
}
func parsePeers(b []byte, local uint32) ([]Peer, error) {
	var rows []struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
		IPv4     string `json:"ipv4"`
		Latency  string `json:"lat_ms"`
		Cost     string `json:"cost"`
		Tunnel   string `json:"tunnel_proto"`
	}
	if json.Unmarshal(b, &rows) != nil || rows == nil || len(rows) > 1024 {
		return nil, errEngine
	}
	peers := []Peer{}
	seen := map[string]bool{}
	for _, r := range rows {
		id, err := strconv.ParseUint(r.ID, 10, 32)
		if err != nil || id == 0 {
			return nil, errEngine
		}
		if uint32(id) == local || r.Cost == "Local" {
			continue
		}
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		p := Peer{ID: r.ID, Hostname: safePeerText(r.Hostname, 128), IPv4: cleanIPv4(r.IPv4), Connection: "unknown"}
		if r.Tunnel != "" && r.Tunnel != "-" {
			p.Connection = "direct"
		} else if strings.Contains(strings.ToLower(r.Cost), "relay") {
			p.Connection = "relay"
		}
		if v, err := strconv.ParseFloat(r.Latency, 64); err == nil && v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) {
			p.LatencyMS = &v
		}
		peers = append(peers, p)
	}
	return peers, nil
}
func cleanIPv4(s string) string {
	if p, err := netip.ParsePrefix(s); err == nil && p.Addr().Is4() {
		return p.Addr().String()
	}
	if p, err := netip.ParseAddr(s); err == nil && p.Is4() {
		return p.String()
	}
	return ""
}
func safePeerText(s string, limit int) string {
	if !validText(s, limit, false) {
		return ""
	}
	return s
}
func privateEnv(dir string) []string {
	// Start from nothing, not os.Environ: ET_*, proxies, loader injection,
	// cloud tokens and agent credentials must not reach either binary.
	env := []string{"HOME=" + dir, "XDG_CONFIG_HOME=" + dir, "XDG_DATA_HOME=" + dir, "XDG_CACHE_HOME=" + dir, "TMPDIR=" + dir, "TMP=" + dir, "TEMP=" + dir, "LANG=C", "RUST_BACKTRACE=0"}
	if runtime.GOOS == "windows" {
		if root := os.Getenv("SystemRoot"); root != "" {
			env = append(env, "SystemRoot="+root, "WINDIR="+root, "PATH="+filepath.Join(root, "System32"))
		}
		env = append(env, "USERPROFILE="+dir, "APPDATA="+dir, "LOCALAPPDATA="+dir)
	} else {
		env = append(env, "PATH=/usr/sbin:/usr/bin:/sbin:/bin")
	}
	return env
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errEngine
	}
	return b.Buffer.Write(p)
}
func runPrivate(ctx context.Context, bin, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = privateEnv(dir)
	cmd.Dir = dir
	cmd.Stderr = io.Discard
	out := &boundedBuffer{limit: 1 << 20}
	cmd.Stdout = out
	cmd.WaitDelay = time.Second
	configureProcess(cmd)
	cmd.Cancel = func() error { return terminateProcess(cmd.Process, true) }
	if err := cmd.Run(); err != nil {
		return nil, errEngine
	}
	return out.Bytes(), nil
}
func (e *Engine) String() string {
	s := e.Snapshot()
	return fmt.Sprintf("EasyTier(mode=%s,status=%s)", s.Mode, s.Status)
}
