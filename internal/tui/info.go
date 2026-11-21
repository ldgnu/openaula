package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/aula"
)

func (m *Model) updateInfo(key string) (tea.Model, tea.Cmd) {
	if key == "esc" || key == "enter" {
		m.screen = screenMenu
	}
	return m, nil
}

func (m *Model) viewInfo() string {
	i := m.info
	var b string

	b += sectionStyle.Render("  Estado del dispositivo") + "\n\n"
	b += fmt.Sprintf("  Conexión:      %s\n", transportName(i.Transport))
	b += fmt.Sprintf("  VID:PID:       %04x:%04x\n", i.VID, i.PID)
	b += fmt.Sprintf("  Producto:      %s\n", i.Product)
	b += fmt.Sprintf("  bcdDevice:     0x%04x\n", i.BCDDevice)
	b += fmt.Sprintf("  Interfaces:    %d\n", i.NumInterfaces)
	b += fmt.Sprintf("  Configurable:  %s\n", yesNo(i.CanConfigure))

	b += "\n" + sectionStyle.Render("  Modelo y layout") + "\n\n"
	b += "  Modelo:        AULA / Epomaker F108 Pro (108 teclas)\n"
	b += "  Layout HID:    ANSI 108, US (no hay variante ISO en este modelo)\n"
	b += "  Fila F:        el firmware arranca en modo multimedia\n"
	b += "                 usá \"F1–F12 directos\" para que manden F1..F12\n"

	b += "\n" + sectionStyle.Render("  Modo del teclado (se cambia en el teclado)") + "\n\n"
	b += "  FN+Q Android · FN+W Windows · FN+E Mac · FN+R iOS\n"
	b += "  FN+~ 2.4G   · FN+1/2/3 Bluetooth · FN+4 USB-C\n"
	b += "  El modo de sistema no se puede leer por HID: lo elige el firmware.\n"

	b += "\n" + dimStyle.Render("  La configuración por USB-C no depende del modo de sistema.") + "\n"
	return b
}

func transportName(t aula.Conn) string {
	switch t {
	case aula.ConnUSB:
		return "USB-C (configuración completa)"
	case aula.Conn24G:
		return "dongle 2.4 GHz (solo lectura)"
	default:
		return "no detectado"
	}
}
