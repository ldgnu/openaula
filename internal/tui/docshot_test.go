package tui

import (
	"strings"
	"testing"
)

func TestEveryScreenRenders(t *testing.T) {
	screens := []string{"menu", "lighting", "lighting-rolling", "perkey", "remap",
		"presets", "os", "wal", "events", "info", "profiles"}
	for _, s := range screens {
		out := DocShot(s, 3)
		if len(strings.TrimSpace(out)) < 20 {
			t.Fatalf("pantalla %q quedó vacía", s)
		}
		if !strings.Contains(out, "openaula") {
			t.Fatalf("pantalla %q no incluye encabezado", s)
		}
	}
}
