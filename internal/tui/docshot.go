package tui

import (
	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/state"
)

// DocShot renders a screen with demo data. It exists so the README screenshots
// can be generated without a keyboard attached.
func DocShot(name string, frame int) string {
	m := &Model{
		settings: state.Default(),
		info: aula.Info{
			Connected:     true,
			Transport:     aula.Conn24G,
			VID:           0x05AC,
			PID:           0x024F,
			Product:       "F108Pro Dongle",
			BCDDevice:     0x0138,
			NumInterfaces: 5,
			CanConfigure:  true,
		},
		width:        100,
		previewFrame: frame,
	}
	m.light = newLightingModel(state.Lighting{Mode: 7, R: 120, G: 70, B: 220, Brightness: 5, Speed: 3})
	m.perkey = newPerKeyModel(state.PerKey{
		Brightness: 5,
		Colors: map[string]state.RGB{
			"w": {0, 255, 0}, "a": {0, 255, 0}, "s": {0, 255, 0}, "d": {0, 255, 0},
			"esc": {255, 0, 80}, "enter": {0, 180, 255},
		},
	})
	m.remap = newRemapModel(state.Remap{})
	m.settings.Remap.Normal = map[string]string{"capslock": "lctrl", "f1": "media:play"}
	m.profiles = newProfilesModel()
	m.profiles.names = []string{"noche", "gentoo"}
	m.profiles.loaded = true

	switch name {
	case "menu":
		m.screen = screenMenu
		m.menuIdx = 0
	case "lighting-rolling":
		m.screen = screenLighting
		m.light = newLightingModel(state.Lighting{Mode: 11, R: 0, G: 180, B: 255, Brightness: 5, Speed: 3})
		m.light.cursor = lfMode
	case "lighting":
		m.screen = screenLighting
		m.light.cursor = lfMode
	case "perkey":
		m.screen = screenPerKey
		m.perkey.row = 3
		m.perkey.col = 1
	case "remap":
		m.screen = screenRemap
		m.remap.cursor = remapRowAdd
	case "presets":
		m.screen = screenPresets
	case "os":
		m.screen = screenOS
		m.os.cursor = 1
	case "wal":
		m.screen = screenWal
	case "info":
		m.screen = screenInfo
	case "profiles":
		m.screen = screenProfiles
	default:
		m.screen = screenMenu
	}
	return m.View()
}
