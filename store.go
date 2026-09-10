package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type diskState struct {
	Version int         `json:"version"`
	Rooms   []savedRoom `json:"rooms"`
}
type Store struct {
	mu    sync.RWMutex
	dir   string
	rooms []savedRoom
	write func(string, []byte) error
}

func privateDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errDisk
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errDisk
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0077 != 0 {
		return errors.New("El directorio de datos debe ser privado (permisos 0700); no se han cambiado sus permisos.")
	}
	return nil
}
func NewStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, errDisk
	}
	if err := privateDir(abs); err != nil {
		return nil, err
	}
	s := &Store{dir: abs, rooms: []savedRoom{}, write: atomicWrite}
	p := filepath.Join(abs, "rooms.json")
	st, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil || !st.Mode().IsRegular() || st.Size() > 1<<20 {
		return nil, errors.New("El archivo de salas no es válido; no se ha sobrescrito.")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(p, 0600); err != nil {
			return nil, errDisk
		}
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, errDisk
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, errDisk
	}
	var data diskState
	if decodeStrict(b, &data) != nil || data.Version != 1 || data.Rooms == nil || len(data.Rooms) > maxRooms {
		return nil, errors.New("El archivo de salas no es válido; no se ha sobrescrito.")
	}
	seen := map[string]bool{}
	for _, r := range data.Rooms {
		if validateRoom(r) != nil || seen[r.ID] {
			return nil, errors.New("El archivo de salas no es válido; no se ha sobrescrito.")
		}
		seen[r.ID] = true
	}
	s.rooms = data.Rooms
	return s, nil
}
func (s *Store) Rooms() []Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		out = append(out, r.Room)
	}
	return out
}
func (s *Store) Get(id string) (savedRoom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.rooms {
		if r.ID == id {
			return r, nil
		}
	}
	return savedRoom{}, errMissing
}
func (s *Store) Add(r savedRoom) (Room, error) {
	if err := validateRoom(r); err != nil {
		return Room{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, old := range s.rooms {
		if old.ID != r.ID {
			continue
		}
		if old.Secret == r.Secret && old.Name == r.Name && old.Game == r.Game && old.Rendezvous == r.Rendezvous && old.RendezvousKey == r.RendezvousKey {
			return old.Room, nil
		}
		return Room{}, errConflict
	}
	if len(s.rooms) >= maxRooms {
		return Room{}, errLimit
	}
	next := append(append([]savedRoom{}, s.rooms...), r)
	if err := s.persist(next); err != nil {
		return Room{}, err
	}
	s.rooms = next
	return r.Room, nil
}
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]savedRoom, 0, len(s.rooms))
	found := false
	for _, r := range s.rooms {
		if r.ID == id {
			found = true
		} else {
			next = append(next, r)
		}
	}
	if !found {
		return errMissing
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.rooms = next
	return nil
}
func (s *Store) persist(rooms []savedRoom) error {
	b, err := json.MarshalIndent(diskState{1, rooms}, "", "  ")
	if err != nil {
		return errDisk
	}
	if err := s.write(filepath.Join(s.dir, "rooms.json"), append(b, '\n')); err != nil {
		return errDisk
	}
	return nil
}

// Rename is the commit point. No error is returned after that point: otherwise
// callers could keep an old in-memory state while the new state is on disk.
func atomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".rooms-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = replaceFile(tmp, path); err != nil {
		return err
	}
	// Best effort directory durability; unsupported on some filesystems/Windows.
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
