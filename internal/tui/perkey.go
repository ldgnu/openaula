package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/state"
)

type perKeyModel struct {
	colors       map[string]state.RGB
	brightness   uint8
	row, col     int
	paletteIdx   int
	pickingColor bool
	pickerIdx    int
}

func newPerKeyModel(pk state.PerKey) perKeyModel {
	colors := make(map[string]state.RGB, len(pk.Colors))
	for k, v := range pk.Colors {
		colors[k] = v
	}
	b := pk.Brightness
	if b == 0 {
		b = 5
	}
	return perKeyModel{colors: colors, brightness: b, paletteIdx: 0}
}

func (m *Model) updatePerKey(key string) (tea.Model, tea.Cmd) {
	p := &m.perkey
	rows := aula.KeyLayout

	if p.pickingColor {
		switch key {
		case "esc":
			p.pickingColor = false
		case "up":
			p.pickerIdx = (p.pickerIdx - 1 + len(palette)) % len(palette)
		case "down":
			p.pickerIdx = (p.pickerIdx + 1) % len(palette)
		case "enter":
			p.paletteIdx = p.pickerIdx
			p.pickingColor = false
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		if p.row > 0 {
			p.row--
			p.col = clamp(p.col, 0, len(rows[p.row])-1)
		}
	case "down":
		if p.row < len(rows)-1 {
			p.row++
			p.col = clamp(p.col, 0, len(rows[p.row])-1)
		}
	case "left":
		p.col = clamp(p.col-1, 0, len(rows[p.row])-1)
	case "right":
		p.col = clamp(p.col+1, 0, len(rows[p.row])-1)
	case "enter":
		p.paint()
	case " ":
		name := rows[p.row][p.col].Name
		delete(p.colors, name)
	case "c":
		p.pickingColor = true
		p.pickerIdx = p.paletteIdx
	case "a":
		m.applyPerKey()
	}
	return m, nil
}

func (p *perKeyModel) paint() {
	name := aula.KeyLayout[p.row][p.col].Name
	c := palette[p.paletteIdx].rgb
	if current, ok := p.colors[name]; ok && current == c {
		delete(p.colors, name)
		return
	}
	p.colors[name] = c
}

func (m *Model) applyPerKey() {
	pk := state.PerKey{Brightness: m.perkey.brightness, Colors: m.perkey.colors}
	if len(pk.Colors) == 0 {
		m.setError(fmt.Errorf("no hay ninguna tecla pintada"))
		return
	}
	if err := m.svc.ApplyPerKey(pk); err != nil {
		m.setError(err)
		return
	}
	m.settings.PerKey = pk
	_ = m.store.Save(m.settings)
	m.setStatus("Aplicado: %d teclas pintadas", len(pk.Colors))
}

func (m *Model) viewPerKey() string {
	p := &m.perkey
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Color por tecla") + "\n")

	color := palette[p.paletteIdx].rgb
	if p.pickingColor {
		b.WriteString("  Elegí color:\n")
		for i, c := range palette {
			cur := "   "
			line := c.name + " " + colorBlock(c.rgb)
			if i == p.pickerIdx {
				cur = " > "
				line = selStyle.Render(" " + c.name + " " + colorBlock(c.rgb) + " ")
			}
			b.WriteString(cur + line + "\n")
		}
		return b.String()
	}

	b.WriteString(fmt.Sprintf("  Color actual: %s %s   (c para cambiar)\n\n", palette[p.paletteIdx].name, colorBlock(color)))

	for r, row := range aula.KeyLayout {
		var line strings.Builder
		line.WriteString("  ")
		for c, k := range row {
			cell := keyCell(k, p.colors[k.Name], r == p.row && c == p.col)
			line.WriteString(cell)
		}
		b.WriteString(line.String() + "\n")
	}

	painted := len(p.colors)
	b.WriteString(fmt.Sprintf("\n  Teclas pintadas: %d · Brillo %d/5\n", painted, p.brightness))
	return b.String()
}

func keyCell(k aula.LayoutKey, c state.RGB, selected bool) string {
	width := k.Width * 5
	label := k.Label
	if len(label) > width-2 {
		label = label[:width-2]
	}
	pad := width - len(label)
	left := pad / 2
	right := pad - left
	text := strings.Repeat(" ", left) + label + strings.Repeat(" ", right)

	set, has := c, c != state.RGB{}
	st := lipgloss.NewStyle()
	if has {
		st = st.
			Background(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]))).
			Foreground(autoContrast(set))
	} else {
		st = st.Foreground(lipgloss.Color("244"))
	}
	if selected {
		st = st.Reverse(true)
	}
	return st.Render(text)
}

func autoContrast(c state.RGB) lipgloss.Color {
	lum := 0.299*float64(c[0]) + 0.587*float64(c[1]) + 0.114*float64(c[2])
	if lum > 140 {
		return lipgloss.Color("16")
	}
	return lipgloss.Color("231")
}
