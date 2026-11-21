package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ldgnu/openaula/internal/state"
)

type lightingModel struct {
	mode       int
	r, g, b    uint8
	brightness uint8
	speed      uint8
	direction  uint8
	colorful   bool
	cursor     int
}

func newLightingModel(l state.Lighting) lightingModel {
	return lightingModel{
		mode:       l.Mode,
		r:          l.R,
		g:          l.G,
		b:          l.B,
		brightness: l.Brightness,
		speed:      l.Speed,
		direction:  l.Direction,
		colorful:   l.Colorful,
	}
}

// lighting fields in display order.
const (
	lfMode = iota
	lfColor
	lfR
	lfG
	lfB
	lfBrightness
	lfSpeed
	lfDirection
	lfColorful
	lfApply
	lfOff
	lfCount
)

// modeInfo describes which fields a mode uses.
func modeInfo(mode int) (color, speed, direction, rainbow bool) {
	switch mode {
	case 0:
		return false, false, false, false
	case 1:
		return true, false, false, false
	case 6, 8:
		return false, true, false, true
	case 10, 11, 12, 16, 18:
		return true, true, true, false
	default:
		return true, true, false, false
	}
}

func (l *lightingModel) fieldVisible(field int) bool {
	color, speed, direction, rainbow := modeInfo(l.mode)
	switch field {
	case lfColor, lfR, lfG, lfB:
		return color && !rainbow
	case lfSpeed:
		return speed
	case lfDirection:
		return direction
	case lfColorful:
		return color && !rainbow
	}
	return true
}

func (l *lightingModel) nextVisible(dir int) {
	for i := 0; i < lfCount; i++ {
		l.cursor += dir
		if l.cursor < 0 {
			l.cursor = lfCount - 1
		}
		if l.cursor >= lfCount {
			l.cursor = 0
		}
		if l.fieldVisible(l.cursor) {
			return
		}
	}
}

func (m *Model) updateLighting(key string) (tea.Model, tea.Cmd) {
	l := &m.light
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		l.nextVisible(-1)
	case "down":
		l.nextVisible(1)
	case "left", "right":
		step := 16
		if key == "left" {
			step = -step
		}
		switch l.cursor {
		case lfMode:
			dir := 1
			if step < 0 {
				dir = -1
			}
			l.mode = (l.mode + dir + 20) % 20
			l.cursor = lfMode
		case lfR:
			l.r = uint8(clamp(int(l.r)+step, 0, 255))
		case lfG:
			l.g = uint8(clamp(int(l.g)+step, 0, 255))
		case lfB:
			l.b = uint8(clamp(int(l.b)+step, 0, 255))
		case lfColor:
			idx := nearestPalette(state.RGB{l.r, l.g, l.b})
			idx += 1
			if step < 0 {
				idx -= 2
			}
			idx = (idx + len(palette) + len(palette)) % len(palette)
			l.r, l.g, l.b = palette[idx].rgb[0], palette[idx].rgb[1], palette[idx].rgb[2]
		case lfBrightness:
			l.brightness = uint8(clamp(int(l.brightness)+step/16, 0, 5))
		case lfSpeed:
			l.speed = uint8(clamp(int(l.speed)+step/16, 0, 5))
		case lfDirection:
			l.direction ^= 1
		case lfColorful:
			l.colorful = !l.colorful
		}
	case "enter":
		switch l.cursor {
		case lfApply:
			m.applyLighting()
		case lfOff:
			m.light.mode = 0
			m.applyLighting()
		case lfColorful:
			l.colorful = !l.colorful
		}
	}
	if !l.fieldVisible(l.cursor) {
		l.nextVisible(1)
	}
	return m, nil
}

func nearestPalette(c state.RGB) int {
	best, bestDist := 0, 1<<30
	for i, p := range palette {
		dr := int(p.rgb[0]) - int(c[0])
		dg := int(p.rgb[1]) - int(c[1])
		db := int(p.rgb[2]) - int(c[2])
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func (m *Model) applyLighting() {
	l := m.light
	lighting := state.Lighting{
		Mode:       l.mode,
		R:          l.r,
		G:          l.g,
		B:          l.b,
		Brightness: l.brightness,
		Speed:      l.speed,
		Direction:  l.direction,
		Colorful:   l.colorful,
	}
	if err := m.svc.ApplyLighting(lighting); err != nil {
		m.setError(err)
		return
	}
	m.settings.Lighting = lighting
	_ = m.store.Save(m.settings)
	m.setStatus("Aplicado: %s", modeName(l.mode))
}

func modeName(id int) string {
	names := []string{
		"Apagado", "Estático", "SingleOn", "SingleOff", "Destellos", "Cayendo",
		"Colorido", "Respirar", "Espectro", "Hacia afuera", "Desplazarse",
		"Rodar", "Rotar", "Explosión", "Lanzamiento", "Ondas",
		"Fluir", "Pulsar", "Inclinación", "Shuttle",
	}
	if id >= 0 && id < len(names) {
		return names[id]
	}
	return "?"
}

func (m *Model) viewLighting() string {
	l := &m.light
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Luz de fondo") + "\n\n")

	row := func(field int, label, value string) {
		if !l.fieldVisible(field) {
			return
		}
		cursor := "   "
		line := fmt.Sprintf("%-12s %s", label, value)
		if l.cursor == field {
			cursor = " > "
			line = selStyle.Render(" " + line + " ")
		}
		b.WriteString(cursor + line + "\n")
	}

	row(lfMode, "Modo", fmt.Sprintf("%-14s (%d/19)", modeName(l.mode), l.mode))
	row(lfColor, "Color", palette[nearestPalette(state.RGB{l.r, l.g, l.b})].name+" "+colorBlock(state.RGB{l.r, l.g, l.b}))
	row(lfR, "Rojo", bar(int(l.r), 255))
	row(lfG, "Verde", bar(int(l.g), 255))
	row(lfB, "Azul", bar(int(l.b), 255))
	row(lfBrightness, "Brillo", bar(int(l.brightness), 5))
	row(lfSpeed, "Velocidad", bar(int(l.speed), 5))
	row(lfDirection, "Dirección", directionName(l.direction))
	row(lfColorful, "Arcoíris", yesNo(l.colorful))

	b.WriteString("\n")
	for _, f := range []int{lfApply, lfOff} {
		label := " Aplicar "
		if f == lfOff {
			label = " Apagar luz "
		}
		if l.cursor == f {
			b.WriteString(" > " + selStyle.Render(label) + "\n")
		} else {
			b.WriteString("   " + label + "\n")
		}
	}

	right := m.renderPreview()
	return lipgloss.JoinHorizontal(lipgloss.Top, b.String(), "    ", right)
}

// renderPreview draws the animated approximation of the selected effect.
func (m *Model) renderPreview() string {
	var out strings.Builder
	out.WriteString(dimStyle.Render("vista previa") + "\n\n")
	for _, row := range previewFrame(m.previewFrame, &m.light) {
		out.WriteString(row + "\n")
	}
	// Leyenda del efecto.
	out.WriteString("\n" + dimStyle.Render(modeName(m.light.mode)))
	return out.String()
}

func directionName(d uint8) string {
	if d == 0 {
		return "Normal"
	}
	return "Invertida"
}

func yesNo(v bool) string {
	if v {
		return "Sí"
	}
	return "No"
}

func bar(value, max int) string {
	width := 20
	filled := 0
	if max > 0 {
		filled = value * width / max
	}
	filled = clamp(filled, 0, width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + fmt.Sprintf("  %d/%d", value, max)
}
