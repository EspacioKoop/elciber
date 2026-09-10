package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxRooms = 32
const maxBody = 16 << 10
const maxInvite = 4096
const invitePrefix = "elciber1:"

var errInvalid = errors.New("Datos no válidos; revisa los campos y sus límites.")
var errDisk = errors.New("No se pudieron guardar los datos de forma segura.")
var errConflict = errors.New("Ya existe una sala con esta identidad y otros datos.")
var errMissing = errors.New("No se encontró la sala.")
var errLimit = errors.New("Se ha alcanzado el límite de 32 salas.")
var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var domainLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

type Room struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Game          string `json:"game"`
	Rendezvous    string `json:"rendezvous"`
	RendezvousKey string `json:"rendezvousKey"`
	CreatedAt     string `json:"createdAt"`
}
type savedRoom struct {
	Room
	Secret string `json:"secret"`
}
type roomInput struct {
	Name          string `json:"name"`
	Game          string `json:"game"`
	Rendezvous    string `json:"rendezvous"`
	RendezvousKey string `json:"rendezvousKey"`
}
type invitation struct {
	Version       int    `json:"version"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Game          string `json:"game"`
	Rendezvous    string `json:"rendezvous"`
	RendezvousKey string `json:"rendezvousKey"`
	Secret        string `json:"secret"`
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("No se pudo obtener aleatoriedad segura.")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func validText(s string, max int, required bool) bool {
	if !utf8.ValidString(s) || len(s) > max || (required && strings.TrimSpace(s) == "") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return false
		}
	}
	return true
}
func validateKey(s string) bool {
	if s == "" {
		return true
	}
	b, err := base64.StdEncoding.Strict().DecodeString(s)
	return err == nil && len(b) == 32 && base64.StdEncoding.EncodeToString(b) == s
}
func validateEndpoint(s string, loopback bool) error {
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "tcp://") && !strings.HasPrefix(s, "udp://") {
		return errInvalid
	}
	if !validText(s, 512, true) || strings.ContainsAny(s, " \\%?#") {
		return errInvalid
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "tcp" && u.Scheme != "udp") || u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return errInvalid
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" || port == "" {
		return errInvalid
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 || strconv.Itoa(p) != port {
		return errInvalid
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() || (loopback && !ip.IsLoopback()) {
			return errInvalid
		}
		return nil
	}
	if loopback || len(host) > 253 {
		return errInvalid
	}
	// Reject legacy/numeric address aliases, which URL parsers may reinterpret.
	if strings.Trim(host, "0123456789.") == "" || strings.HasPrefix(strings.ToLower(host), "0x") {
		return errInvalid
	}
	for _, label := range strings.Split(host, ".") {
		if !domainLabel.MatchString(label) {
			return errInvalid
		}
	}
	return nil
}
func (in roomInput) validate() error {
	if !validText(in.Name, 80, true) || !validText(in.Game, 80, false) || !validateKey(in.RendezvousKey) {
		return errInvalid
	}
	return validateEndpoint(in.Rendezvous, false)
}
func validateRoom(r savedRoom) error {
	if !idPattern.MatchString(r.ID) {
		return errInvalid
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(r.Secret)
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != r.Secret {
		return errInvalid
	}
	if err := (roomInput{r.Name, r.Game, r.Rendezvous, r.RendezvousKey}).validate(); err != nil {
		return err
	}
	if len(r.CreatedAt) > 40 {
		return errInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err != nil {
		return errInvalid
	}
	return nil
}
func createRoom(in roomInput) (savedRoom, error) {
	if err := in.validate(); err != nil {
		return savedRoom{}, err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return savedRoom{}, errInvalid
	}
	secret, err := randomToken(32)
	if err != nil {
		return savedRoom{}, err
	}
	return savedRoom{Room{hex.EncodeToString(id), in.Name, in.Game, in.Rendezvous, in.RendezvousKey, time.Now().UTC().Format(time.RFC3339Nano)}, secret}, nil
}
func encodeInvite(r savedRoom) (string, error) {
	if err := validateRoom(r); err != nil {
		return "", err
	}
	b, err := json.Marshal(invitation{1, r.ID, r.Name, r.Game, r.Rendezvous, r.RendezvousKey, r.Secret})
	if err != nil {
		return "", errInvalid
	}
	return invitePrefix + base64.RawURLEncoding.EncodeToString(b), nil
}
func decodeInvite(s string) (savedRoom, error) {
	if len(s) > maxInvite || !strings.HasPrefix(s, invitePrefix) {
		return savedRoom{}, errInvalid
	}
	payload := strings.TrimPrefix(s, invitePrefix)
	b, err := base64.RawURLEncoding.Strict().DecodeString(payload)
	if err != nil || base64.RawURLEncoding.EncodeToString(b) != payload {
		return savedRoom{}, errInvalid
	}
	var v invitation
	if err := decodeStrict(b, &v); err != nil || v.Version != 1 {
		return savedRoom{}, errInvalid
	}
	r := savedRoom{Room{v.ID, v.Name, v.Game, v.Rendezvous, v.RendezvousKey, time.Now().UTC().Format(time.RFC3339Nano)}, v.Secret}
	return r, validateRoom(r)
}

// Reject duplicate keys, unknown/case-folded fields, null, trailing values and
// excessive nesting rather than relying on encoding/json's permissive defaults.
func decodeStrict(b []byte, dst any) error {
	if len(b) > 1<<20 || !utf8.Valid(b) {
		return errInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := checkJSONValue(d, 0); err != nil {
		return errInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return errInvalid
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return errInvalid
	}
	if !exactJSONFields(value, reflect.TypeOf(dst).Elem()) {
		return errInvalid
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return errInvalid
	}
	return nil
}
func checkJSONValue(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errInvalid
	}
	t, err := d.Token()
	if err != nil || t == nil {
		return errInvalid
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return errInvalid
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return errInvalid
			}
			seen[s] = true
			if err := checkJSONValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkJSONValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errInvalid
	}
	_, err = d.Token()
	return err
}
func jsonFields(t reflect.Type) map[string]reflect.Type {
	m := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			for k, v := range jsonFields(f.Type) {
				m[k] = v
			}
			continue
		}
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			m[tag] = f.Type
		}
	}
	return m
}
func exactJSONFields(v any, t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		fields := jsonFields(t)
		for k, x := range m {
			ft, ok := fields[k]
			if !ok || !exactJSONFields(x, ft) {
				return false
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return false
		}
		for _, x := range a {
			if !exactJSONFields(x, t.Elem()) {
				return false
			}
		}
	}
	return v != nil
}
