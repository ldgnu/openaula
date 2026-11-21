package tui

import (
	"strings"
	"testing"
)

func TestPreviewRendersEveryMode(t *testing.T) {
	for mode := 0; mode <= 19; mode++ {
		l := &lightingModel{mode: mode, r: 255, g: 40, b: 90, brightness: 4, speed: 3}
		rows := previewFrame(7, l)
		if len(rows) != prevH {
			t.Fatalf("mode %d: %d filas", mode, len(rows))
		}
		for _, r := range rows {
			if strings.Count(r, "  ") < prevW {
				t.Fatalf("mode %d: fila incompleta", mode)
			}
		}
	}
}

func TestHueWrap(t *testing.T) {
	for _, h := range []float64{-0.5, 0, 0.25, 0.75, 1.5} {
		c := hueRGB(h)
		for _, v := range c {
			if v < 0 || v > 255 {
				t.Fatalf("hue %v -> %v", h, c)
			}
		}
	}
}
