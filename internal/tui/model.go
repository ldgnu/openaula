// Package tui implements the interactive terminal interface.
//
// Navigation is deliberately simple: arrows to move, left/right to change a
// value, Enter to act, Esc to go back. Every screen shows its own hints.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/device"
	"github.com/ldgnu/openaula/internal/state"
)

type screen int

const (
	screenMenu screen = iota
	screenLighting
	screenPerKey
	screenRemap
	screenClock
	screenLCD
	screenProfiles
	screenPresets
	screenOS
	screenInfo
)

// Model is the root bubbletea model.
type Model struct {
	svc   *device.Service
	store *state.Store

	settings state.Settings
	info     aula.Info

	screen  screen
	menuIdx int

	light    lightingModel
	perkey   perKeyModel
	remap    remapModel
	presets  presetsModel
	os       osModel
	lcd      lcdModel
	profiles profilesModel

	width  int
	height int

	status string
	errMsg string
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selStyle     = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("230"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

// New creates the TUI model.
func New(svc *device.Service, store *state.Store) *Model {
	st, err := store.Load()
	if err != nil {
		st = state.Default()
	}
	m := &Model{
		svc:      svc,
		store:    store,
		settings: st,
		info:     aula.Detect(),
	}
	m.light = newLightingModel(st.Lighting)
	m.perkey = newPerKeyModel(st.PerKey)
	m.remap = newRemapModel(st.Remap)
	m.lcd = newLCDModel()
	m.profiles = newProfilesModel()
	return m
}

// Run starts the interface.
func Run(svc *device.Service, store *state.Store) error {
	p := tea.NewProgram(New(svc, store), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Text inputs consume their own keys.
	if cmd, handled := m.inputActive(msg); handled {
		return m, cmd
	}

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenMenu:
		return m.updateMenu(key)
	case screenLighting:
		return m.updateLighting(key)
	case screenPerKey:
		return m.updatePerKey(key)
	case screenRemap:
		return m.updateRemap(msg)
	case screenClock:
		return m.updateClock(key)
	case screenLCD:
		return m.updateLCD(key)
	case screenProfiles:
		return m.updateProfiles(key)
	case screenPresets:
		return m.updatePresets(key)
	case screenOS:
		return m.updateOS(key)
	case screenInfo:
		return m.updateInfo(key)
	}
	return m, nil
}

var menuItems = []struct {
	label  string
	screen screen
}{
	{"Luz de fondo", screenLighting},
	{"Color por tecla", screenPerKey},
	{"Remapear teclas", screenRemap},
	{"Ajustes rápidos (F1–F12, Enter)", screenPresets},
	{"Sistema operativo", screenOS},
	{"Reloj del LCD", screenClock},
	{"Pantalla del LCD", screenLCD},
	{"Perfiles", screenProfiles},
	{"Estado del dispositivo", screenInfo},
}

func (m *Model) updateMenu(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.menuIdx > 0 {
			m.menuIdx--
		}
	case "down", "j":
		if m.menuIdx < len(menuItems)-1 {
			m.menuIdx++
		}
	case "enter":
		m.screen = menuItems[m.menuIdx].screen
		m.errMsg = ""
		m.status = ""
		if m.screen == screenInfo {
			m.info = aula.Detect()
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" openaula "))
	b.WriteString("  ")
	b.WriteString(m.deviceBadge())
	b.WriteString("\n\n")

	switch m.screen {
	case screenMenu:
		b.WriteString(m.viewMenu())
	case screenLighting:
		b.WriteString(m.viewLighting())
	case screenPerKey:
		b.WriteString(m.viewPerKey())
	case screenRemap:
		b.WriteString(m.viewRemap())
	case screenClock:
		b.WriteString(m.viewClock())
	case screenLCD:
		b.WriteString(m.viewLCD())
	case screenProfiles:
		b.WriteString(m.viewProfiles())
	case screenInfo:
		b.WriteString(m.viewInfo())
	}

	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(errStyle.Render("  " + m.errMsg))
	} else if m.status != "" {
		b.WriteString(okStyle.Render("  " + m.status))
	} else {
		b.WriteString(" ")
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  " + m.hints()))
	b.WriteString("\n")
	return b.String()
}

func (m *Model) deviceBadge() string {
	switch m.info.Transport {
	case aula.ConnUSB:
		return okStyle.Render("USB-C conectado")
	case aula.Conn24G:
		return errStyle.Render("solo dongle 2.4G — configurar por cable USB-C")
	default:
		return errStyle.Render("sin teclado")
	}
}

func (m *Model) viewMenu() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Menú") + "\n\n")
	for i, item := range menuItems {
		cursor := "  "
		line := item.label
		if i == m.menuIdx {
			cursor = "> "
			line = selStyle.Render(" " + line + " ")
		}
		b.WriteString(cursor + line + "\n")
	}
	return b.String()
}

func (m *Model) hints() string {
	switch m.screen {
	case screenMenu:
		return "↑↓ mover · Enter entrar · q salir"
	case screenLighting:
		return "↑↓ campo · ←→ ajustar · Enter en Aplicar/Apagar · Esc volver"
	case screenPerKey:
		if m.perkey.pickingColor {
			return "↑↓ color · Enter elegir · Esc cancelar"
		}
		return "↑↓←→ mover · Enter pintar · Espacio borrar · c color · a aplicar · Esc volver"
	case screenRemap:
		return m.remap.hint()
	case screenClock:
		return "Enter sincronizar · Esc volver"
	case screenLCD:
		return m.lcd.hint()
	case screenProfiles:
		return m.profiles.hint()
	case screenPresets:
		return "↑↓ elegir · Enter aplicar · Esc volver"
	case screenOS:
		return "↑↓ elegir · Enter aplicar · Esc volver"
	case screenInfo:
		return "Enter o Esc volver"
	}
	return ""
}

// ---- LED preview ----

var palette = []struct {
	name string
	rgb  state.RGB
}{
	{"Rojo", state.RGB{255, 0, 0}},
	{"Naranja", state.RGB{255, 128, 0}},
	{"Amarillo", state.RGB{255, 255, 0}},
	{"Verde", state.RGB{0, 255, 0}},
	{"Cian", state.RGB{0, 255, 255}},
	{"Azul", state.RGB{0, 80, 255}},
	{"Violeta", state.RGB{160, 0, 255}},
	{"Rosa", state.RGB{255, 0, 160}},
	{"Blanco", state.RGB{255, 255, 255}},
}

func colorBlock(c state.RGB) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]))).
		Render("    ")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Model) setStatus(format string, args ...any) {
	m.status = fmt.Sprintf(format, args...)
	m.errMsg = ""
}

func (m *Model) setError(err error) {
	m.errMsg = err.Error()
	m.status = ""
}

// inputActive routes keys to an active text input when present.
func (m *Model) inputActive(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.screen == screenLCD && m.lcd.input.Focused() {
		if msg.String() == "enter" {
			return m.lcd.submitPath(), true
		}
		if msg.String() == "esc" {
			m.lcd.closeInput()
			return nil, true
		}
		var cmd tea.Cmd
		m.lcd.input, cmd = m.lcd.input.Update(msg)
		return cmd, true
	}
	if m.screen == screenProfiles && m.profiles.input.Focused() {
		if msg.String() == "enter" {
			return m.profiles.submit(m), true
		}
		if msg.String() == "esc" {
			m.profiles.closeInput()
			return nil, true
		}
		var cmd tea.Cmd
		m.profiles.input, cmd = m.profiles.input.Update(msg)
		return cmd, true
	}
	return nil, false
}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 200
	return ti
}
