package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/state"
	"github.com/ldgnu/openaula/internal/wal"
)

type walModel struct {
	pal   wal.Palette
	err   error
	index int
}

func (m *Model) updateWal(key string) (tea.Model, tea.Cmd) {
	w := &m.wal
	switch key {
	case "esc":
		m.screen = screenMenu
	case "left":
		w.index = (w.index - 1 + 16) % 16
	case "right":
		w.index = (w.index + 1) % 16
	case "r":
		w.load()
	case "enter":
		c := w.pal.Accent(w.index)
		l := m.settings.Lighting
		if l.Mode == 0 {
			l.Mode = 1
		}
		l.R, l.G, l.B = c[0], c[1], c[2]
		if err := m.svc.ApplyLighting(l); err != nil {
			m.setError(err)
			return m, nil
		}
		m.settings.Lighting = l
		_ = m.store.Save(m.settings)
		m.light = newLightingModel(l)
		m.setStatus("Color pywal %d aplicado: #%02x%02x%02x", w.index, c[0], c[1], c[2])
	}
	return m, nil
}

func (w *walModel) load() {
	pal, err := wal.Load()
	w.pal = pal
	w.err = err
}

func (m *Model) viewWal() string {
	w := &m.wal
	if w.pal.Path == "" && w.err == nil {
		w.load()
	}
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Color de pywal") + "\n\n")
	if w.err != nil {
		b.WriteString(errStyle.Render("  "+w.err.Error()) + "\n")
		b.WriteString(dimStyle.Render("  Corré wal sobre un wallpaper y volvé con r.") + "\n")
		return b.String()
	}
	b.WriteString(dimStyle.Render("  "+w.pal.Path) + "\n\n")
	for i := 0; i < 16; i++ {
		c := w.pal.Colors[i]
		line := fmt.Sprintf("color%-2d %s #%02x%02x%02x", i, colorBlock(state.RGB{c[0], c[1], c[2]}), c[0], c[1], c[2])
		if i == w.index {
			b.WriteString(" > " + selStyle.Render(" "+line+" ") + "\n")
		} else {
			b.WriteString("   " + line + "\n")
		}
	}
	b.WriteString("\n" + dimStyle.Render("  Enter aplica el color elegido con el modo y brillo actuales.") + "\n")
	return b.String()
}
