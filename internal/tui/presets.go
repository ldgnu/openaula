package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/device"
)

type presetsModel struct {
	cursor int
}

func (m *Model) updatePresets(key string) (tea.Model, tea.Cmd) {
	list := device.Presets()
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		m.presets.cursor = (m.presets.cursor - 1 + len(list)) % len(list)
	case "down":
		m.presets.cursor = (m.presets.cursor + 1) % len(list)
	case "enter":
		p := list[m.presets.cursor]
		merged := map[string]string{}
		existing := m.settings.Remap.Normal
		if p.Layer == "fn" {
			existing = m.settings.Remap.FN
		}
		for k, v := range existing {
			merged[k] = v
		}
		for k, v := range p.Keys {
			merged[k] = v
		}
		if err := m.svc.ApplyRemap(p.Layer, merged); err != nil {
			m.setError(err)
			return m, nil
		}
		if p.Layer == "fn" {
			m.settings.Remap.FN = merged
		} else {
			m.settings.Remap.Normal = merged
		}
		_ = m.store.Save(m.settings)
		m.setStatus("Aplicado: %s", p.Name)
	}
	return m, nil
}

func (m *Model) viewPresets() string {
	list := device.Presets()
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Ajustes rápidos") + "\n\n")
	for i, p := range list {
		cur := "   "
		name := p.Name
		if i == m.presets.cursor {
			cur = " > "
			name = selStyle.Render(" " + name + " ")
		}
		b.WriteString(cur + name + "\n")
		if i == m.presets.cursor {
			for _, line := range wrap(p.Description, 72) {
				b.WriteString("     " + dimStyle.Render(line) + "\n")
			}
		}
	}
	b.WriteString("\n" + dimStyle.Render("  Se suman a los remapeos que ya tengas en esa capa.") + "\n")
	return b.String()
}

func wrap(s string, width int) []string {
	words := strings.Fields(s)
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len()+len(w)+1 > width && cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteString(" ")
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

var _ = fmt.Sprintf
