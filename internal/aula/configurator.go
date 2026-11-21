package aula

import "time"

// Configurator is the set of operations both transports implement.
type Configurator interface {
	SetLighting(LightingConfig) error
	SyncClock(time.Time) error
	SetPerKeyRGB([]KeyColor, uint8) error
	SetKeyRemap([]KeyRemap) error
	SetFnKeyRemap([]KeyRemap) error
	ResetKeyRemap() error
	ResetFnKeyRemap() error
}
