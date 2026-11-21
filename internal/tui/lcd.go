package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	lcdRowColor = iota
	lcdRowImage
	lcdRowClock
	lcdRowBack
	lcdRowCount
)

type lcdModel struct {
	cursor int
	input  textinput.Model
	// picking is true while choosing a solid color.
	picking bool
	pickIdx int
	pending string
}

func newLCDModel() lcdModel {
	l := lcdModel{}
	l.input = newTextInput("/ruta/a/animacion.gif")
	l.input.Prompt = "  GIF: "
	return l
}

func (l *lcdModel) closeInput() {
	l.input.Blur()
	l.input.SetValue("")
}

func (l *lcdModel) submitPath() tea.Cmd {
	l.pending = strings.TrimSpace(l.input.Value())
	return func() tea.Msg { return lcdUploadMsg{path: l.pending} }
}

type lcdUploadMsg struct{ path string }

func (m *Model) updateLCD(key string) (tea.Model, tea.Cmd) {
	l := &m.lcd

	// Confirm step for image upload.
	if l.pending != "" && !l.input.Focused() {
		switch key {
		case "enter":
			path := l.pending
			l.pending = ""
			m.uploadLCD(path)
		case "esc":
			l.pending = ""
			m.setStatus("Subida cancelada")
		}
		return m, nil
	}

	if l.picking {
		switch key {
		case "esc":
			l.picking = false
		case "up":
			l.pickIdx = (l.pickIdx - 1 + len(palette)) % len(palette)
		case "down":
			l.pickIdx = (l.pickIdx + 1) % len(palette)
		case "enter":
			c := palette[l.pickIdx].rgb
			l.picking = false
			if err := m.svc.LCDSolid(c[0], c[1], c[2]); err != nil {
				m.setError(err)
				return m, nil
			}
			m.setStatus("LCD pintado de %s", palette[l.pickIdx].name)
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.screen = screenMenu
	case "up", "down":
		dir := 1
		if key == "up" {
			dir = -1
		}
		l.cursor = (l.cursor + dir + lcdRowCount) % lcdRowCount
	case "enter":
		switch l.cursor {
		case lcdRowColor:
			l.picking = true
			l.pickIdx = 4
		case lcdRowImage:
			l.input.Focus()
		case lcdRowClock:
			if err := m.svc.SyncClock(); err != nil {
				m.setError(err)
				return m, nil
			}
			m.setStatus("Reloj sincronizado")
		case lcdRowBack:
			m.screen = screenMenu
		}
	}
	return m, nil
}

func (m *Model) uploadLCD(path string) {
	f, err := os.Open(path)
	if err != nil {
		m.setError(err)
		return
	}
	defer f.Close()
	if err := m.svc.LCDImage(f, nil); err != nil {
		m.setError(err)
		return
	}
	m.setStatus("Imagen subida al LCD")
}

func (m *Model) viewLCD() string {
	l := &m.lcd
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Pantalla del LCD") + "\n\n")

	if l.pending != "" {
		b.WriteString("  " + errStyle.Render("Atención:") + " subir imágenes puede dañar los gráficos\n")
		b.WriteString("  internos del teclado de forma permanente.\n\n")
		b.WriteString(fmt.Sprintf("  Archivo: %s\n\n", l.pending))
		b.WriteString("  Enter para continuar · Esc para cancelar\n")
		return b.String()
	}

	if l.picking {
		b.WriteString("  Color sólido:\n\n")
		for i, c := range palette {
			cur := "   "
			line := c.name + " " + colorBlock(c.rgb)
			if i == l.pickIdx {
				cur = " > "
				line = selStyle.Render(" " + c.name + " " + colorBlock(c.rgb) + " ")
			}
			b.WriteString(cur + line + "\n")
		}
		return b.String()
	}

	rows := []string{
		"Color sólido",
		"Subir imagen GIF (riesgoso)",
		"Sincronizar reloj",
		"Volver",
	}
	for i, row := range rows {
		cur := "   "
		if i == l.cursor {
			cur = " > "
			row = selStyle.Render(" " + row + " ")
		}
		b.WriteString(cur + row + "\n")
	}

	if l.input.Focused() {
		b.WriteString("\n" + l.input.View() + "\n")
		b.WriteString(dimStyle.Render("  Enter aceptar · Esc cancelar") + "\n")
	}
	return b.String()
}

func (l *lcdModel) hint() string {
	switch {
	case l.pending != "":
		return "Enter confirmar · Esc cancelar"
	case l.picking:
		return "↑↓ color · Enter pintar · Esc volver"
	}
	return "↑↓ mover · Enter elegir · Esc volver"
}
