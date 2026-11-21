// Package state holds the desired keyboard configuration and named profiles.
//
// The keyboard protocol is write-only: it cannot be queried for its current
// lighting, per-key colors or remaps. openaula therefore keeps the settings it
// last applied here and treats this file as the source of truth.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RGB is a color in 0-255 components.
type RGB [3]uint8

// Lighting describes a backlight effect.
type Lighting struct {
	Mode       int   `json:"mode"`
	R          uint8 `json:"r"`
	G          uint8 `json:"g"`
	B          uint8 `json:"b"`
	Brightness uint8 `json:"brightness"`
	Speed      uint8 `json:"speed"`
	Direction  uint8 `json:"direction"`
	Colorful   bool  `json:"colorful"`
}

// PerKey describes a per-key RGB layout.
type PerKey struct {
	Brightness uint8          `json:"brightness"`
	Colors     map[string]RGB `json:"colors"`
}

// Remap maps source key names to targets, per layer.
type Remap struct {
	Normal map[string]string `json:"normal"`
	FN     map[string]string `json:"fn"`
}

// Settings is the full desired configuration.
type Settings struct {
	Lighting Lighting  `json:"lighting"`
	PerKey   PerKey    `json:"per_key"`
	Remap    Remap     `json:"remap"`
	OS       string    `json:"os"`
	Updated  time.Time `json:"updated"`
}

// Default returns sensible defaults matching the keyboard's factory look.
func Default() Settings {
	return Settings{
		Lighting: Lighting{Mode: 11, R: 255, G: 0, B: 0, Brightness: 5, Speed: 3},
		PerKey:   PerKey{Brightness: 5, Colors: map[string]RGB{}},
		Remap:    Remap{Normal: map[string]string{}, FN: map[string]string{}},
		OS:       "windows",
	}
}

// Store persists settings and profiles under a config directory.
type Store struct {
	dir string
}

// NewStore returns a store rooted at $XDG_CONFIG_HOME/openaula.
func NewStore() (*Store, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "openaula")
	if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the configuration directory.
func (s *Store) Dir() string { return s.dir }

// Load reads the current settings, returning defaults when no file exists.
func (s *Store) Load() (Settings, error) {
	path := filepath.Join(s.dir, "state.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	var st Settings
	if err := json.Unmarshal(data, &st); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	normalize(&st)
	return st, nil
}

// Save writes the settings atomically.
func (s *Store) Save(st Settings) error {
	st.Updated = time.Now()
	normalize(&st)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "state.json"), data)
}

// ListProfiles returns the profile names, sorted.
func (s *Store) ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, "profiles"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name()[:len(e.Name())-len(".json")])
	}
	sort.Strings(names)
	return names, nil
}

// SaveProfile stores a copy of the settings under name.
func (s *Store) SaveProfile(name string, st Settings) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "profiles", name+".json"), data)
}

// LoadProfile reads a named profile.
func (s *Store) LoadProfile(name string) (Settings, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "profiles", name+".json"))
	if err != nil {
		return Settings{}, err
	}
	var st Settings
	if err := json.Unmarshal(data, &st); err != nil {
		return Settings{}, err
	}
	normalize(&st)
	return st, nil
}

// DeleteProfile removes a named profile.
func (s *Store) DeleteProfile(name string) error {
	return os.Remove(filepath.Join(s.dir, "profiles", name+".json"))
}

func normalize(st *Settings) {
	if st.PerKey.Colors == nil {
		st.PerKey.Colors = map[string]RGB{}
	}
	if st.Remap.Normal == nil {
		st.Remap.Normal = map[string]string{}
	}
	if st.Remap.FN == nil {
		st.Remap.FN = map[string]string{}
	}
	if st.Lighting.Mode < 0 || st.Lighting.Mode > 19 {
		st.Lighting.Mode = 1
	}
	if st.OS == "" {
		st.OS = "windows"
	}
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
