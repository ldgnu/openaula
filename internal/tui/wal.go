package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/state"
	"github.com/ldgnu/openaula/internal/wal"
)

type walModel struct {
	pal    wal.Palette
	err    error
	index  int
	cursor int
}

func (w *walModel) load(m *Model) {
	pal, err := wal.Load()
	w.pal = pal
	w.err = err
	w.index = m.settings.WalIndex
	if w.index < 0 || w.index > 15 {
		w.index = 4
	}
}

func (m *Model) updateWal(key string) (tea.Model, tea.Cmd) {
	w := &m.wal
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		w.cursor = (w.cursor - 1 + 17) % 17
	case "down":
		w.cursor = (w.cursor + 1) % 17
	case "left":
		if w.cursor > 0 {
			w.index = (w.index - 1 + 16) % 16
			m.settings.WalIndex = w.index
			_ = m.store.Save(m.settings)
		}
	case "right":
		if w.cursor > 0 {
			w.index = (w.index + 1) % 16
			m.settings.WalIndex = w.index
			_ = m.store.Save(m.settings)
		}
	case "r":
		w.load(m)
		m.setStatus("Paleta recargada")
	case "enter":
		if w.cursor == 0 {
			m.settings.FollowWal = !m.settings.FollowWal
			_ = m.store.Save(m.settings)
			if m.settings.FollowWal {
				m.setStatus("Seguir color de pywal activado")
			} else {
				m.setStatus("Seguir color de pywal desactivado")
			}
			return m, nil
		}
		m.applyWalColor()
	}
	return m, nil
}

func (m *Model) applyWalColor() {
	w := &m.wal
	c := w.pal.Accent(w.index)
	l := m.settings.Lighting
	if l.Mode == 0 {
		l.Mode = 1
	}
	l.R, l.G, l.B = c[0], c[1], c[2]
	if err := m.svc.ApplyLighting(l); err != nil {
		m.setError(err)
		return
	}
	m.settings.Lighting = l
	m.settings.WalIndex = w.index
	_ = m.store.Save(m.settings)
	m.light = newLightingModel(l)
	m.setStatus("Color pywal %d aplicado: #%02x%02x%02x", w.index, c[0], c[1], c[2])
}

// applyWalOnStart applies the saved pywal color when the option is enabled.
func (m *Model) applyWalOnStart() {
	if !m.settings.FollowWal {
		return
	}
	pal, err := wal.Load()
	if err != nil {
		return
	}
	m.wal.pal = pal
	m.wal.index = m.settings.WalIndex
	c := pal.Accent(m.settings.WalIndex)
	l := m.settings.Lighting
	if l.Mode == 0 {
		l.Mode = 1
	}
	l.R, l.G, l.B = c[0], c[1], c[2]
	if err := m.svc.ApplyLighting(l); err != nil {
		return
	}
	m.settings.Lighting = l
	_ = m.store.Save(m.settings)
	m.light = newLightingModel(l)
}

func (m *Model) viewWal() string {
	w := &m.wal
	if w.pal.Path == "" && w.err == nil {
		w.load(m)
	}
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Color de pywal") + "\n\n")

	follow := "Desactivado"
	if m.settings.FollowWal {
		follow = "Activado"
	}
	row := func(cursor int, label string) string {
		if cursor == w.cursor {
			return " > " + selStyle.Render(" "+label+" ") + "\n"
		}
		return "   " + label + "\n"
	}
	b.WriteString(row(0, fmt.Sprintf("Seguir color de pywal al iniciar: %s", follow)))
	b.WriteString("\n")

	if w.err != nil {
		b.WriteString(errStyle.Render("  "+w.err.Error()) + "\n")
		b.WriteString(dimStyle.Render("  Corré wal sobre un wallpaper y volvé con r.\n"))
		return b.String()
	}
	b.WriteString(dimStyle.Render("  "+w.pal.Path) + "\n\n")
	for i := 0; i < 16; i++ {
		c := w.pal.Colors[i]
		line := fmt.Sprintf("color%-2d %s #%02x%02x%02x", i, colorBlock(state.RGB{c[0], c[1], c[2]}), c[0], c[1], c[2])
		b.WriteString(row(i+1, line))
	}
	b.WriteString("\n" + dimStyle.Render("  Enter aplica o cambia la opción · ←→ elige color · r recarga") + "\n")
	return b.String()
}
