package device

import (
	"fmt"

	"github.com/ldgnu/openaula/internal/aula"
)

// OSProfile is a remap bundle that matches a host OS layout.
//
// The keyboard's own device mode (FN+Q Android, FN+W Windows, FN+E Mac,
// FN+R iOS) lives in the firmware and cannot be set over HID. What we can do
// is make the normal layer produce the codes the host OS expects, which is
// what these profiles do.
type OSProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	FNCombo     string            `json:"fn_combo"`
	Keys        map[string]string `json:"keys"`
}

// OSProfiles returns the supported host profiles.
func OSProfiles() []OSProfile {
	return []OSProfile{
		{
			ID:          "windows",
			Name:        "Windows / Linux",
			Description: "Disposición de PC: Alt a los costados de la barra espaciadora y Win/Super donde corresponde.",
			FNCombo:     "FN+W",
			Keys: map[string]string{
				"lalt": "lalt", "ralt": "ralt",
				"lwin": "lwin", "rctrl": "rctrl", "lctrl": "lctrl",
			},
		},
		{
			ID:          "macos",
			Name:        "macOS",
			Description: "Intercambia Alt y Command para que coincidan con un teclado de Mac. Elegí también FN+E en el teclado.",
			FNCombo:     "FN+E",
			Keys: map[string]string{
				"lalt": "lwin", "lwin": "lalt",
				"ralt": "rwin",
			},
		},
		{
			ID:          "ios",
			Name:        "iPadOS / iOS",
			Description: "Igual que macOS para los modificadores. Elegí FN+R en el teclado.",
			FNCombo:     "FN+R",
			Keys: map[string]string{
				"lalt": "lwin", "lwin": "lalt",
				"ralt": "rwin",
			},
		},
		{
			ID:          "android",
			Name:        "Android",
			Description: "Disposición de PC. Elegí FN+Q en el teclado.",
			FNCombo:     "FN+Q",
			Keys: map[string]string{
				"lalt": "lalt", "ralt": "ralt", "lwin": "lwin",
			},
		},
		{
			ID:          "neutral",
			Name:        "Sin cambios",
			Description: "No toca los modificadores. Usalo si ya te anda bien tal cual viene.",
			FNCombo:     "",
			Keys:        map[string]string{},
		},
	}
}

// OSProfileByID finds a host profile.
func OSProfileByID(id string) (OSProfile, error) {
	for _, p := range OSProfiles() {
		if p.ID == id {
			return p, nil
		}
	}
	return OSProfile{}, fmt.Errorf("perfil de sistema desconocido %q", id)
}

// ApplyOSProfile merges the profile's remaps into the existing normal-layer
// remaps and sends the full layer to the keyboard.
func (s *Service) ApplyOSProfile(id string, existing map[string]string) (map[string]string, error) {
	p, err := OSProfileByID(id)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]string, len(existing)+len(p.Keys))
	for k, v := range existing {
		keys[k] = v
	}
	for k, v := range p.Keys {
		keys[k] = v
	}
	if err := s.ApplyRemap("normal", keys); err != nil {
		return nil, err
	}
	return keys, nil
}

var _ = aula.KeyNameToHID
