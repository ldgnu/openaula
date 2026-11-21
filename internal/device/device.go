// Package device applies configurations to the keyboard.
//
// Every operation opens the USB device, sends the command sequence and closes
// it again. There is no background process and no polling: when nothing is
// running, openaula consumes zero CPU, memory and USB bandwidth.
package device

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/state"
)

// ErrNotConfigured means a 2.4 GHz dongle was found but no wired keyboard.
var ErrNotConfigured = fmt.Errorf("keyboard not connected via USB-C; configuration over the 2.4 GHz dongle is not supported")

// Service serializes access to the single keyboard.
type Service struct {
	mu sync.Mutex
}

// New returns a Service.
func New() *Service { return &Service{} }

// Status returns device detection plus the last applied settings.
func (s *Service) Status(store *state.Store) (Status, error) {
	st, err := store.Load()
	if err != nil {
		return Status{}, err
	}
	info := aula.Detect()
	return Status{
		Device:   info,
		Settings: st,
		Writable: info.Transport == aula.ConnUSB,
	}, nil
}

// Status is the payload returned by GET /api/v1/status.
type Status struct {
	Device   aula.Info      `json:"device"`
	Settings state.Settings `json:"settings"`
	Writable bool           `json:"writable"`
}

// withDevice opens the wired keyboard, runs fn and closes it.
func (s *Service) withDevice(fn func(*aula.Device) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := aula.Detect()
	switch info.Transport {
	case aula.ConnUSB:
	case aula.Conn24G:
		return ErrNotConfigured
	default:
		return fmt.Errorf("keyboard not found")
	}

	dev, err := aula.Open()
	if err != nil {
		return err
	}
	defer dev.Close()
	return fn(dev)
}

// ApplyLighting sends a backlight effect.
func (s *Service) ApplyLighting(l state.Lighting) error {
	if l.Mode < 0 || l.Mode > 19 {
		return fmt.Errorf("mode out of range (0-19)")
	}
	cfg := aula.LightingConfig{
		Mode:       aula.LightingMode(l.Mode),
		R:          l.R,
		G:          l.G,
		B:          l.B,
		Brightness: l.Brightness,
		Speed:      l.Speed,
		Direction:  l.Direction,
		Colorful:   l.Colorful,
	}
	return s.withDevice(func(d *aula.Device) error { return d.SetLighting(cfg) })
}

// ApplyPerKey sends a per-key RGB layout. Keys missing from the map are off.
func (s *Service) ApplyPerKey(pk state.PerKey) error {
	keys := make([]aula.KeyColor, 0, len(pk.Colors))
	for name, rgb := range pk.Colors {
		idx, ok := aula.KeyNameToIndex[strings.ToLower(name)]
		if !ok {
			return fmt.Errorf("unknown key %q", name)
		}
		keys = append(keys, aula.KeyColor{LightIndex: idx, R: rgb[0], G: rgb[1], B: rgb[2]})
	}
	brightness := pk.Brightness
	if brightness > 5 {
		brightness = 5
	}
	return s.withDevice(func(d *aula.Device) error { return d.SetPerKeyRGB(keys, brightness) })
}

// ApplyRemap sends the normal and/or FN remap tables.
func (s *Service) ApplyRemap(layer string, entries map[string]string) error {
	remaps := make([]aula.KeyRemap, 0, len(entries))
	for src, target := range entries {
		srcIdx, ok := aula.KeyNameToIndex[strings.ToLower(src)]
		if !ok {
			return fmt.Errorf("unknown source key %q", src)
		}
		r, err := ParseRemapTarget(srcIdx, target)
		if err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		remaps = append(remaps, r)
	}

	var fn bool
	switch strings.ToLower(layer) {
	case "", "normal":
	case "fn":
		fn = true
	default:
		return fmt.Errorf("unknown layer %q (use normal or fn)", layer)
	}

	return s.withDevice(func(d *aula.Device) error {
		if fn {
			return d.SetFnKeyRemap(remaps)
		}
		return d.SetKeyRemap(remaps)
	})
}

// ResetRemap clears a remap layer.
func (s *Service) ResetRemap(layer string) error {
	fn := strings.EqualFold(layer, "fn")
	return s.withDevice(func(d *aula.Device) error {
		if fn {
			return d.ResetFnKeyRemap()
		}
		return d.ResetKeyRemap()
	})
}

// SyncClock sets the LCD clock to the current local time.
func (s *Service) SyncClock() error {
	now := time.Now()
	return s.withDevice(func(d *aula.Device) error { return d.SyncClock(now) })
}

// LCDSolid fills the LCD with a solid color.
func (s *Service) LCDSolid(r, g, b uint8) error {
	buf := aula.BuildSolidLCDImage(r, g, b)
	return s.withDevice(func(d *aula.Device) error { return d.UploadLCDImage(buf, 1, nil) })
}

// LCDImage uploads a GIF, PNG or JPEG to the LCD.
func (s *Service) LCDImage(r io.Reader, progress func(sent, total int)) error {
	buf, _, err := aula.BuildAnyLCDImage(r)
	if err != nil {
		return err
	}
	return s.withDevice(func(d *aula.Device) error { return d.UploadLCDImage(buf, 1, progress) })
}

// ParseRemapTarget converts a target spec such as "a", "media:play",
// "mouse:lclick" or "combo:ctrl+c" into a KeyRemap for the given source index.
func ParseRemapTarget(srcIdx uint8, spec string) (aula.KeyRemap, error) {
	spec = strings.ToLower(strings.TrimSpace(spec))

	switch {
	case strings.HasPrefix(spec, "media:"):
		code, ok := aula.ConsumerNameToCode[strings.TrimPrefix(spec, "media:")]
		if !ok {
			return aula.KeyRemap{}, fmt.Errorf("unknown media action %q", spec)
		}
		return aula.NewConsumerRemap(srcIdx, code), nil
	case strings.HasPrefix(spec, "mouse:"):
		params, ok := aula.MouseNameToParams[strings.TrimPrefix(spec, "mouse:")]
		if !ok {
			return aula.KeyRemap{}, fmt.Errorf("unknown mouse action %q", spec)
		}
		return aula.NewMouseRemap(srcIdx, params[0], params[1], params[2]), nil
	case strings.HasPrefix(spec, "combo:"):
		return parseCombo(srcIdx, strings.TrimPrefix(spec, "combo:"))
	}

	hid, ok := aula.KeyNameToHID[spec]
	if !ok {
		return aula.KeyRemap{}, fmt.Errorf("unknown key %q", spec)
	}
	return aula.NewKeySwap(srcIdx, hid), nil
}

func parseCombo(srcIdx uint8, expr string) (aula.KeyRemap, error) {
	modMap := map[string]byte{
		"ctrl": 0x01, "shift": 0x02, "alt": 0x04, "win": 0x08,
		"rctrl": 0x10, "rshift": 0x20, "ralt": 0x40, "rwin": 0x80,
	}
	parts := strings.Split(expr, "+")
	if len(parts) < 2 {
		return aula.KeyRemap{}, fmt.Errorf("combo must look like ctrl+c")
	}
	var bits byte
	for _, p := range parts[:len(parts)-1] {
		bit, ok := modMap[p]
		if !ok {
			return aula.KeyRemap{}, fmt.Errorf("unknown modifier %q", p)
		}
		bits |= bit
	}
	hid, ok := aula.KeyNameToHID[parts[len(parts)-1]]
	if !ok {
		return aula.KeyRemap{}, fmt.Errorf("unknown key %q in combo", parts[len(parts)-1])
	}
	return aula.NewKeyCombo(srcIdx, bits, hid), nil
}
