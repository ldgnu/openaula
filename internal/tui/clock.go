package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateClock(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenMenu
	case "enter":
		if err := m.svc.SyncClock(); err != nil {
			m.setError(err)
			return m, nil
		}
		m.setStatus("Reloj del LCD sincronizado")
	}
	return m, nil
}

func (m *Model) viewClock() string {
	now := time.Now()
	return sectionStyle.Render("  Reloj del LCD") + "\n\n" +
		fmt.Sprintf("  Hora del sistema: %s\n", now.Format("2006-01-02 15:04:05")) + "\n" +
		"  Enter para enviarla al teclado.\n"
}
