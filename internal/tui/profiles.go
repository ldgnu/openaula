package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type profilesModel struct {
	names  []string
	cursor int
	input  textinput.Model
	saving bool
	loaded bool
}

func newProfilesModel() profilesModel {
	p := profilesModel{}
	p.input = newTextInput("nombre del perfil")
	p.input.Prompt = "  Nombre: "
	return p
}

func (p *profilesModel) refresh(m *Model) {
	names, err := m.store.ListProfiles()
	if err != nil {
		m.setError(err)
		return
	}
	p.names = names
	p.loaded = true
	if p.cursor >= len(names)+2 {
		p.cursor = 0
	}
}

func (p *profilesModel) closeInput() {
	p.input.Blur()
	p.input.SetValue("")
	p.saving = false
}

func (p *profilesModel) submit(m *Model) tea.Cmd {
	name := strings.TrimSpace(p.input.Value())
	if name == "" {
		return nil
	}
	if err := m.store.SaveProfile(name, m.settings); err != nil {
		m.setError(err)
		return nil
	}
	p.closeInput()
	p.refresh(m)
	m.setStatus("Perfil guardado: %s", name)
	return nil
}

func (m *Model) updateProfiles(key string) (tea.Model, tea.Cmd) {
	p := &m.profiles
	if !p.loaded {
		p.refresh(m)
	}

	if p.saving {
		return m, nil
	}

	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		p.cursor--
	case "down":
		p.cursor++
	case "enter":
		if p.cursor == 0 {
			p.saving = true
			p.input.Focus()
			return m, nil
		}
		idx := p.cursor - 1
		if idx < len(p.names) {
			m.applyProfile(p.names[idx])
		}
	case "d":
		idx := p.cursor - 1
		if idx >= 0 && idx < len(p.names) {
			name := p.names[idx]
			if err := m.store.DeleteProfile(name); err != nil {
				m.setError(err)
				return m, nil
			}
			p.refresh(m)
			m.setStatus("Perfil borrado: %s", name)
		}
	}

	rows := len(p.names) + 2
	if p.cursor < 0 {
		p.cursor = rows - 1
	}
	if p.cursor >= rows {
		p.cursor = 0
	}
	return m, nil
}

func (m *Model) applyProfile(name string) {
	st, err := m.store.LoadProfile(name)
	if err != nil {
		m.setError(err)
		return
	}
	if st.Lighting.Mode > 0 {
		if err := m.svc.ApplyLighting(st.Lighting); err != nil {
			m.setError(err)
			return
		}
	}
	if len(st.PerKey.Colors) > 0 {
		if err := m.svc.ApplyPerKey(st.PerKey); err != nil {
			m.setError(err)
			return
		}
	}
	if len(st.Remap.Normal) > 0 {
		if err := m.svc.ApplyRemap("normal", st.Remap.Normal); err != nil {
			m.setError(err)
			return
		}
	}
	if len(st.Remap.FN) > 0 {
		if err := m.svc.ApplyRemap("fn", st.Remap.FN); err != nil {
			m.setError(err)
			return
		}
	}
	m.settings = st
	_ = m.store.Save(st)
	m.light = newLightingModel(st.Lighting)
	m.perkey = newPerKeyModel(st.PerKey)
	m.setStatus("Perfil aplicado: %s", name)
}

func (m *Model) viewProfiles() string {
	p := &m.profiles
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Perfiles") + "\n\n")

	rows := []string{"Guardar configuración actual"}
	rows = append(rows, p.names...)
	rows = append(rows, "Volver")

	for i, row := range rows {
		cur := "   "
		line := row
		if i == p.cursor {
			cur = " > "
			line = selStyle.Render(" " + line + " ")
		}
		b.WriteString(cur + line + "\n")
	}
	if len(p.names) == 0 {
		b.WriteString(dimStyle.Render("\n   todavía no hay perfiles guardados") + "\n")
	}
	if p.saving {
		b.WriteString("\n" + p.input.View() + "\n")
		b.WriteString(dimStyle.Render("  Enter guardar · Esc cancelar") + "\n")
	}
	return b.String()
}

func (p *profilesModel) hint() string {
	if p.saving {
		return "Enter guardar · Esc cancelar"
	}
	return "↑↓ mover · Enter actuar · d borrar perfil · Esc volver"
}

var _ = fmt.Sprintf
