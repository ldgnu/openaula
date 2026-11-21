package tui

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"
)

const (
	prevW = 15
	prevH = 6
)

// previewFrame renders an ASCII approximation of the selected lighting effect.
// It is not a copy of the firmware animation, just a reference so you can see
// what the mode does before applying it.
func previewFrame(frame int, l *lightingModel) []string {
	color, _, _, rainbow := modeInfo(l.mode)
	useRainbow := rainbow || l.colorful
	speed := 0.4 + float64(l.speed)*0.35

	base := [3]float64{float64(l.r), float64(l.g), float64(l.b)}
	rows := make([]string, prevH)
	cx, cy := float64(prevW-1)/2, float64(prevH-1)/2

	for y := 0; y < prevH; y++ {
		line := ""
		for x := 0; x < prevW; x++ {
			v := cellIntensity(l.mode, x, y, frame, speed, cx, cy)
			c := base
			if useRainbow || !color {
				hue := (float64(x)/prevW + float64(y)/prevH*0.35 + float64(frame)*0.03*speed)
				c = hueRGB(hue)
			}
			if l.mode == 0 {
				v = 0.03
			}
			c = [3]float64{
				c[0] * v * (0.35 + float64(l.brightness)*0.13),
				c[1] * v * (0.35 + float64(l.brightness)*0.13),
				c[2] * v * (0.35 + float64(l.brightness)*0.13),
			}
			line += lipgloss.NewStyle().
				Background(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", clamp255(c[0]), clamp255(c[1]), clamp255(c[2])))).
				Render("  ")
		}
		rows[y] = line
	}
	return rows
}

func clamp255(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v)
}

// cellIntensity returns 0..1 for a cell of the preview grid.
func cellIntensity(mode, x, y, frame int, speed, cx, cy float64) float64 {
	f := float64(frame) * speed
	dx := float64(x) - cx
	dy := float64(y) - cy
	dist := math.Hypot(dx, dy)

	switch mode {
	case 0:
		return 0
	case 1:
		return 1
	case 2: // SingleOn
		if x == frame%prevW && y == (frame/prevW)%prevH {
			return 1
		}
		return 0.08
	case 3: // SingleOff
		if x == frame%prevW && y == (frame/prevW)%prevH {
			return 0.06
		}
		return 0.9
	case 4: // Glittering
		h := math.Sin(float64(x*928371+frame*7919)*0.017) * math.Cos(float64(y*1237+frame*104729)*0.021)
		if h > 0.55 {
			return 1
		}
		return 0.08
	case 5: // Falling
		head := math.Mod(f+float64(x)*1.7, prevH+3)
		if math.Abs(float64(y)-head) < 0.8 {
			return 1
		}
		if float64(y) > head-3 && float64(y) < head {
			return 0.35
		}
		return 0.06
	case 6, 8: // Colourful / Spectrum
		return 0.55 + 0.45*math.Sin(f*0.6+float64(x)*0.4+float64(y)*0.5)
	case 7: // Breath
		return 0.25 + 0.75*(0.5-0.5*math.Cos(f*0.9))
	case 9: // Outward
		ring := math.Mod(f*1.4, 12)
		if math.Abs(dist-ring) < 1.1 {
			return 1
		}
		return 0.07
	case 10: // Scrolling
		head := math.Mod(f*2.2, prevW+6)
		if math.Abs(float64(x)-head) < 1 {
			return 1
		}
		if float64(x) < head && float64(x) > head-4 {
			return 0.3
		}
		return 0.06
	case 11, 12, 16, 18: // Rolling / Rotating / Flowing / Tilt
		return 0.2 + 0.8*(0.5-0.5*math.Cos(f*0.8+float64(y)*0.9))
	case 13, 14: // Explode / Launch
		ring := math.Mod(f*1.7, 13)
		if math.Abs(dist-ring) < 1.2 {
			return 1 - ring/13
		}
		return 0.07
	case 15: // Ripples
		w := math.Sin(dist*1.1 - f*1.3)
		return 0.15 + 0.85*(0.5+0.5*w)
	case 17: // Pulsating
		return 0.2 + 0.8*(0.5-0.5*math.Cos(f*1.6))
	case 19: // Shuttle
		pos := math.Mod(f*2.5, float64(prevW*2-2))
		if pos > float64(prevW-1) {
			pos = float64(prevW*2-2) - pos
		}
		if math.Abs(float64(x)-pos) < 1.2 {
			return 1
		}
		return 0.07
	}
	return 0.4
}

// hueRGB converts 0..1 hue to an RGB triplet.
func hueRGB(h float64) [3]float64 {
	h = math.Mod(h, 1)
	if h < 0 {
		h += 1
	}
	r := math.Abs(h*6-3) - 1
	g := 2 - math.Abs(h*6-2)
	b := 2 - math.Abs(h*6-4)
	clamp := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	return [3]float64{clamp(r) * 255, clamp(g) * 255, clamp(b) * 255}
}
