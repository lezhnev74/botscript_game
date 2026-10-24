// Package store is the file-based app.Store: board.json (the all-time
// leaderboard), bots.json (the players' bots and tokens) and map.txt (the map
// the admin applied, plain ASCII), last.json (the final board of the latest
// finished game) in one directory.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/lezhnev/arena/engine/app"
)

const (
	boardFile = "board.json"
	botsFile  = "bots.json"
	mapFile   = "map.txt"
	lastFile  = "last.json"
)

// Store keeps its data in Dir. Writes are atomic (temp file in the same
// directory, fsync, rename), so a crash leaves the old or the new file, never
// half of one. bots.json holds the secret tokens and is private (0600).
type Store struct {
	dir string
	mu  sync.Mutex // one writer at a time
}

// New returns a Store for dir; the directory is created on the first save.
func New(dir string) *Store { return &Store{dir: dir} }

// LoadBoard implements app.Store.
func (s *Store) LoadBoard() ([]app.Entry, error) {
	var out []app.Entry
	err := s.load(boardFile, &out)
	return out, err
}

// SaveBoard implements app.Store.
func (s *Store) SaveBoard(e []app.Entry) error { return s.save(boardFile, e, 0o644) }

// LoadLast implements app.Store.
func (s *Store) LoadLast() ([]app.Entry, error) {
	var out []app.Entry
	err := s.load(lastFile, &out)
	return out, err
}

// SaveLast implements app.Store.
func (s *Store) SaveLast(e []app.Entry) error { return s.save(lastFile, e, 0o644) }

// LoadBots implements app.Store.
func (s *Store) LoadBots() ([]app.SavedBot, error) {
	var out []app.SavedBot
	err := s.load(botsFile, &out)
	return out, err
}

// SaveBots implements app.Store.
func (s *Store) SaveBots(b []app.SavedBot) error { return s.save(botsFile, b, 0o600) }

// LoadMap implements app.Store: "" when no map was ever saved.
func (s *Store) LoadMap() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, mapFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// SaveMap implements app.Store.
func (s *Store) SaveMap(text string) error { return s.write(mapFile, []byte(text), 0o644) }

func (s *Store) load(name string, into any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func (s *Store) save(name string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.write(name, data, perm)
}

func (s *Store) write(name string, data []byte, perm os.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, name), data, perm)
}

// writeAtomic writes data to path through a temp file and a rename.
func writeAtomic(path string, data []byte, perm os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name()) //nolint:errcheck // best effort
		}
	}()
	if err = tmp.Chmod(perm); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
