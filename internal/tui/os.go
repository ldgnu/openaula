package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/device"
)

type osModel struct {
	cursor int
}

func (m *Model) updateOS(key string) (tea.Model, tea.Cmd) {
	list := device.OSProfiles()
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		m.os.cursor = (m.os.cursor - 1 + len(list)) % len(list)
	case "down":
		m.os.cursor = (m.os.cursor + 1) % len(list)
	case "enter":
		p := list[m.os.cursor]
		keys, err := m.svc.ApplyOSProfile(p.ID, m.settings.Remap.Normal)
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.settings.Remap.Normal = keys
		m.settings.OS = p.ID
		_ = m.store.Save(m.settings)
		msg := "Perfil aplicado: " + p.Name
		if p.FNCombo != "" {
			msg += " · acordate de elegir " + p.FNCombo + " en el teclado"
		}
		m.setStatus("%s", msg)
	}
	return m, nil
}

func (m *Model) viewOS() string {
	list := device.OSProfiles()
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Sistema operativo") + "\n\n")
	b.WriteString(dimStyle.Render("  Ajusta los modificadores para el sistema que uses.") + "\n")
	b.WriteString(dimStyle.Render("  El modo interno del teclado se elige con FN+Q/W/E/R.") + "\n\n")
	for i, p := range list {
		cur := "   "
		line := p.Name
		if p.ID == m.settings.OS {
			line += okStyle.Render("  (actual)")
		}
		if i == m.os.cursor {
			cur = " > "
			line = selStyle.Render(" " + p.Name + " ")
			if p.ID == m.settings.OS {
				line += okStyle.Render("  (actual)")
			}
		}
		b.WriteString(cur + line + "\n")
		if i == m.os.cursor {
			for _, l := range wrap(p.Description, 70) {
				b.WriteString("     " + dimStyle.Render(l) + "\n")
			}
			if p.FNCombo != "" {
				b.WriteString("     " + dimStyle.Render("combo en el teclado: "+p.FNCombo) + "\n")
			}
		}
	}
	return b.String()
}
