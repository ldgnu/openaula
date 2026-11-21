package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/state"
)

const (
	remapRowAdd = iota
	remapRowLayer
	remapRowApply
	remapRowReset
	remapRowBack
)

type remapModel struct {
	layer int // 0 normal, 1 FN.
	// pending selections.
	src string
	dst string

	cursor  int
	listOff int

	stage int // 0 list, 1 pick source, 2 pick target.

	filter  textinput.Model
	targets []string
	keys    []string
	sel     int
	off     int
}

func newRemapModel(r state.Remap) remapModel {
	keys := make([]string, 0, len(aula.KeyNameToIndex))
	for name := range aula.KeyNameToIndex {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	var targets []string
	targets = append(targets, keys...)
	for _, extra := range []string{
		"media:play", "media:next", "media:prev", "media:volup", "media:voldown", "media:mute",
		"mouse:lclick", "mouse:rclick", "mouse:mclick", "mouse:scrollup", "mouse:scrolldn",
		"combo:ctrl+c", "combo:ctrl+v", "combo:ctrl+x", "combo:ctrl+z", "combo:win+d", "combo:alt+tab",
	} {
		targets = append(targets, extra)
	}

	rm := remapModel{keys: keys, targets: targets}
	rm.filter = newTextInput("filtrar…")
	rm.filter.Prompt = "  "
	return rm
}

func (r *remapModel) layerName() string {
	if r.layer == 1 {
		return "FN"
	}
	return "Normal"
}

func (m *Model) updateRemap(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r := &m.remap
	key := msg.String()

	if r.stage != 0 {
		if r.filter.Focused() && key != "esc" && key != "enter" && key != "tab" && key != "up" && key != "down" {
			var cmd tea.Cmd
			r.filter, cmd = r.filter.Update(msg)
			r.sel = 0
			r.clampSel()
			return m, cmd
		}
		switch key {
		case "esc":
			r.closePicker()
		case "up":
			r.sel--
		case "down":
			r.sel++
		case "enter", "tab":
			m.remapPick()
		}
		r.clampSel()
		return m, nil
	}

	rows := len(m.remapRows())
	switch key {
	case "esc":
		m.screen = screenMenu
	case "up":
		r.cursor = (r.cursor - 1 + rows) % rows
	case "down":
		r.cursor = (r.cursor + 1) % rows
	case "left", "right":
		if r.cursor == remapRowLayer {
			r.layer ^= 1
		}
	case "enter":
		m.remapEnter()
	}
	return m, nil
}

// remapRows returns display rows for the remap screen.
func (m *Model) remapRows() []string {
	r := &m.remap
	entries := m.currentRemap()
	rows := []string{"+ Agregar remapeo"}
	names := make([]string, 0, len(entries))
	for k := range entries {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		rows = append(rows, fmt.Sprintf("%s  ->  %s", k, entries[k]))
	}
	rows = append(rows, "Aplicar cambios", "Restablecer capa "+r.layerName(), "Volver")
	return rows
}

func (m *Model) currentRemap() map[string]string {
	if m.remap.layer == 1 {
		return m.settings.Remap.FN
	}
	return m.settings.Remap.Normal
}

func (m *Model) setCurrentRemap(v map[string]string) {
	if m.remap.layer == 1 {
		m.settings.Remap.FN = v
	} else {
		m.settings.Remap.Normal = v
	}
}

func (m *Model) remapEnter() {
	r := &m.remap
	entries := m.currentRemap()
	entryRows := len(entries)
	switch {
	case r.cursor == remapRowAdd:
		r.stage = 1
		r.sel, r.off = 0, 0
		r.filter.SetValue("")
		r.filter.Focus()
	case r.cursor < remapRowAdd+1+entryRows:
		// Delete the selected entry.
		names := make([]string, 0, len(entries))
		for k := range entries {
			names = append(names, k)
		}
		sort.Strings(names)
		src := names[r.cursor-1]
		delete(entries, src)
		m.setCurrentRemap(entries)
		m.setStatus("Quitado: %s", src)
	case r.cursor == remapRowAdd+1+entryRows:
		if err := m.svc.ApplyRemap(r.layerName(), entries); err != nil {
			m.setError(err)
			return
		}
		_ = m.store.Save(m.settings)
		m.setStatus("Remapeos aplicados en capa %s", r.layerName())
	case r.cursor == remapRowAdd+2+entryRows:
		if err := m.svc.ResetRemap(r.layerName()); err != nil {
			m.setError(err)
			return
		}
		m.setCurrentRemap(map[string]string{})
		_ = m.store.Save(m.settings)
		m.setStatus("Capa %s restablecida", r.layerName())
	case r.cursor == remapRowAdd+3+entryRows:
		m.screen = screenMenu
	}
}

func (r *remapModel) closePicker() {
	r.stage = 0
	r.src, r.dst = "", ""
	r.filter.Blur()
}

func (r *remapModel) filtered() []string {
	q := strings.ToLower(strings.TrimSpace(r.filter.Value()))
	src := r.keys
	if r.stage == 2 {
		src = r.targets
	}
	if q == "" {
		return src
	}
	var out []string
	for _, s := range src {
		if strings.Contains(strings.ToLower(s), q) {
			out = append(out, s)
		}
	}
	return out
}

func (r *remapModel) clampSel() {
	items := r.filtered()
	if len(items) == 0 {
		r.sel = 0
		return
	}
	if r.sel < 0 {
		r.sel = len(items) - 1
	}
	if r.sel >= len(items) {
		r.sel = 0
	}
}

func (m *Model) remapPick() {
	r := &m.remap
	items := r.filtered()
	if len(items) == 0 {
		return
	}
	if r.sel >= len(items) {
		r.sel = len(items) - 1
	}
	choice := items[r.sel]
	if r.stage == 1 {
		r.src = choice
		r.stage = 2
		r.sel, r.off = 0, 0
		r.filter.SetValue("")
		return
	}
	// Validate the target before adding it.
	idx, ok := aula.KeyNameToIndex[r.src]
	if !ok {
		m.setError(fmt.Errorf("tecla origen desconocida: %s", r.src))
		return
	}
	if _, err := m.parseTarget(idx, choice); err != nil {
		m.setError(err)
		return
	}
	entries := m.currentRemap()
	entries[r.src] = choice
	m.setCurrentRemap(entries)
	m.setStatus("Agregado: %s -> %s", r.src, choice)
	r.closePicker()
}

func (m *Model) viewRemap() string {
	r := &m.remap
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Remapear teclas") + "\n\n")

	if r.stage != 0 {
		title := "Elegí la tecla de origen"
		if r.stage == 2 {
			title = fmt.Sprintf("%s  ->  elegí el destino", r.src)
		}
		b.WriteString("  " + sectionStyle.Render(title) + "\n")
		b.WriteString(r.filter.View() + "\n\n")
		items := r.filtered()
		visible := 12
		if r.sel < r.off {
			r.off = r.sel
		}
		if r.sel >= r.off+visible {
			r.off = r.sel - visible + 1
		}
		for i := r.off; i < len(items) && i < r.off+visible; i++ {
			cur := "   "
			line := items[i]
			if i == r.sel {
				cur = " > "
				line = selStyle.Render(" " + line + " ")
			}
			b.WriteString(cur + line + "\n")
		}
		if len(items) == 0 {
			b.WriteString(dimStyle.Render("   sin resultados") + "\n")
		}
		return b.String()
	}

	rows := m.remapRows()
	for i, row := range rows {
		cur := "   "
		line := row
		switch {
		case i == remapRowLayer:
			line = fmt.Sprintf("Capa: %s   (←→)", r.layerName())
		case i >= remapRowAdd+1 && i < len(rows)-3:
			line = "  " + row + dimStyle.Render("   (Enter quita)")
		}
		if i == r.cursor {
			cur = " > "
			line = selStyle.Render(" " + line + " ")
		}
		b.WriteString(cur + line + "\n")
	}
	return b.String()
}

func (r *remapModel) hint() string {
	if r.stage != 0 {
		return "escribí para filtrar · ↑↓ elegir · Enter aceptar · Esc cancelar"
	}
	return "↑↓ mover · ←→ cambiar capa · Enter actuar · Esc volver"
}

// parseTarget is wired to the device package; declared here to keep imports tidy.
func (m *Model) parseTarget(idx uint8, spec string) (any, error) {
	return m.svc.ParseTarget(idx, spec)
}
