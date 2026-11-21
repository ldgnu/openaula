package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/events"
)

type eventsModel struct {
	cursor  int
	running bool
	cancel  context.CancelFunc
	errMsg  string
}

const (
	evNotifications = iota
	evVolume
	evBattery
	evFullCharge
	evSleep
	evLock
	evLEDs
	evSchedule
	evToggle
	evBack
	evCount
)

func (m *Model) updateEvents(key string) (tea.Model, tea.Cmd) {
	e := &m.events
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		e.cursor = (e.cursor - 1 + evCount) % evCount
	case "down":
		e.cursor = (e.cursor + 1) % evCount
	case "left", "right":
		if e.cursor < evToggle {
			m.toggleEvent(e.cursor)
		}
	case "enter":
		if e.cursor < evToggle {
			m.toggleEvent(e.cursor)
		} else if e.cursor == evToggle {
			if e.running {
				m.stopEvents()
			} else {
				m.startEvents()
			}
		} else {
			m.screen = screenMenu
		}
	}
	return m, nil
}

func (m *Model) toggleEvent(row int) {
	ev := &m.settings.Events
	switch row {
	case evNotifications:
		ev.Notifications = !ev.Notifications
	case evVolume:
		ev.Volume = !ev.Volume
	case evBattery:
		ev.Battery = !ev.Battery
	case evFullCharge:
		ev.FullCharge = !ev.FullCharge
	case evSleep:
		ev.Sleep = !ev.Sleep
	case evLock:
		ev.Lock = !ev.Lock
	case evLEDs:
		ev.LEDs = !ev.LEDs
	case evSchedule:
		ev.Schedule = !ev.Schedule
	}
	_ = m.store.Save(m.settings)
	if m.events.running {
		m.setStatus("Opción cambiada; reiniciá la vigilancia para aplicarla")
	}
}

func (m *Model) startEvents() {
	ctx, cancel := context.WithCancel(context.Background())
	m.events.cancel = cancel

	o := events.Opts{
		Notifications: m.settings.Events.Notifications,
		NotifColor:    [3]uint8{255, 255, 255},
		Volume:        m.settings.Events.Volume,
		Up:            [3]uint8{0, 255, 0},
		Down:          [3]uint8{255, 0, 0},
		Battery:       m.settings.Events.Battery,
		Threshold:     m.settings.Events.Threshold,
		LowColor:      [3]uint8{255, 0, 0},
		Sleep:         m.settings.Events.Sleep,
		Lock:          m.settings.Events.Lock,
		LockColor:     [3]uint8{255, 120, 0},
		FullCharge:    m.settings.Events.FullCharge,
		FullLevel:     m.settings.Events.FullLevel,
		FullColor:     [3]uint8{0, 255, 0},
		LEDs:          m.settings.Events.LEDs,
		Schedule:      m.settings.Events.Schedule,
		DayStart:      m.settings.Events.DayStart,
		DayEnd:        m.settings.Events.DayEnd,
		DayColor:      [3]uint8{200, 220, 255},
		NightColor:    [3]uint8{255, 140, 60},
	}
	if !o.Any() {
		m.setError(fmt.Errorf("activá al menos un evento"))
		return
	}
	m.events.running = true
	m.events.errMsg = ""
	go func() {
		if err := events.Run(ctx, m.svc, m.store, o); err != nil {
			m.events.errMsg = err.Error()
		}
	}()
	m.setStatus("Vigilancia activa")
}

func (m *Model) stopEvents() {
	if m.events.cancel != nil {
		m.events.cancel()
		m.events.cancel = nil
	}
	m.events.running = false
	m.setStatus("Vigilancia detenida")
}

func (m *Model) viewEvents() string {
	e := &m.events
	ev := m.settings.Events
	rows := []struct {
		label string
		on    bool
	}{
		{"Notificaciones", ev.Notifications},
		{"Cambios de volumen", ev.Volume},
		{"Batería baja", ev.Battery},
		{"Carga completa", ev.FullCharge},
		{"Suspensión", ev.Sleep},
		{"Bloqueo de sesión", ev.Lock},
		{"Caps Lock / Num Lock", ev.LEDs},
		{"Perfil por hora", ev.Schedule},
	}

	out := sectionStyle.Render("  Eventos del sistema") + "\n\n"
	out += dimStyle.Render(fmt.Sprintf("  Batería baja: %d%% · carga completa: %d%% · día %02d-%02d",
		ev.Threshold, ev.FullLevel, ev.DayStart, ev.DayEnd)) + "\n\n"

	for i, r := range rows {
		state := "No"
		if r.on {
			state = "Sí"
		}
		line := fmt.Sprintf("%-22s %s", r.label, state)
		if i == e.cursor {
			out += " > " + selStyle.Render(" "+line+" ") + "\n"
		} else {
			out += "   " + line + "\n"
		}
	}

	toggle := "Iniciar vigilancia"
	if e.running {
		toggle = "Detener vigilancia"
	}
	if e.cursor == evToggle {
		out += "\n > " + selStyle.Render(" "+toggle+" ") + "\n"
	} else {
		out += "\n   " + toggle + "\n"
	}
	if e.running {
		out += okStyle.Render("   activa") + "\n"
	}
	if e.errMsg != "" {
		out += errStyle.Render("   "+e.errMsg) + "\n"
	}
	if e.cursor == evBack {
		out += " > " + selStyle.Render(" Volver ") + "\n"
	} else {
		out += "   Volver\n"
	}
	return out
}
