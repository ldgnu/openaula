package device

import (
	"fmt"

	"github.com/ldgnu/openaula/internal/aula"
)

// Preset is a named remap bundle that fixes common annoyances.
type Preset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Layer       string            `json:"layer"`
	Keys        map[string]string `json:"keys"`
	// Append keeps the existing remaps instead of replacing the layer.
	Append bool `json:"append"`
}

// Presets returns the built-in remap presets.
func Presets() []Preset {
	directF := map[string]string{}
	for _, k := range []string{
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
	} {
		directF[k] = k
	}

	return []Preset{
		{
			ID:          "fkeys",
			Name:        "F1–F12 directos",
			Description: "Hace que F1..F12 manden la tecla de función sin apretar Fn (los atajos quedan en Fn+F1..F12).",
			Layer:       "normal",
			Keys:        directF,
			Append:      true,
		},
		{
			ID:          "enter",
			Name:        "Restaurar Enter",
			Description: "Fuerza que la tecla Intro mande Enter (HID 0x28) en la capa normal.",
			Layer:       "normal",
			Keys:        map[string]string{"enter": "enter"},
			Append:      true,
		},
		{
			ID:          "win-linux",
			Name:        "Modo Windows/Linux",
			Description: "Deja Alt y Win/Command en su posición estándar de PC.",
			Layer:       "normal",
			Keys:        map[string]string{"lalt": "lalt", "lwin": "lwin", "ralt": "ralt"},
			Append:      true,
		},
		{
			ID:          "mac",
			Name:        "Modo Mac (compensación)",
			Description: "Si usás el teclado en modo Mac (FN+E), intercambia Alt y Win para que coincidan con la disposición de macOS.",
			Layer:       "normal",
			Keys:        map[string]string{"lalt": "lwin", "lwin": "lalt", "ralt": "rwin"},
			Append:      true,
		},
	}
}

// PresetByID finds a preset.
func PresetByID(id string) (Preset, error) {
	for _, p := range Presets() {
		if p.ID == id {
			return p, nil
		}
	}
	return Preset{}, fmt.Errorf("unknown preset %q", id)
}

// ApplyPreset applies a preset, optionally merging with the existing remaps.
func (s *Service) ApplyPreset(id string, appendExisting bool) (map[string]string, error) {
	p, err := PresetByID(id)
	if err != nil {
		return nil, err
	}
	keys := map[string]string{}
	if appendExisting {
		for k, v := range p.Keys {
			keys[k] = v
		}
	}
	if err := s.ApplyRemap(p.Layer, keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// ParseTarget exposes remap target validation to the TUI.
func (s *Service) ParseTarget(srcIdx uint8, spec string) (aula.KeyRemap, error) {
	return ParseRemapTarget(srcIdx, spec)
}
