package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ldgnu/openaula/internal/device"
	"github.com/ldgnu/openaula/internal/state"
)

// flasher applies a short lighting burst and restores the saved configuration.
type flasher struct {
	svc   *device.Service
	store *state.Store

	mu       sync.Mutex
	color    [3]uint8
	duration time.Duration
}

// burst flashes the keyboard n times, for alerts that must not be missed.
func (f *flasher) burst(c [3]uint8, n int) {
	for i := 0; i < n; i++ {
		f.flash(c)
	}
}

func (f *flasher) flash(c [3]uint8) {
	f.mu.Lock()
	defer f.mu.Unlock()

	st, err := f.store.Load()
	if err != nil {
		return
	}
	restore := st.Lighting

	burst := state.Lighting{
		Mode:       1, // Static.
		R:          c[0],
		G:          c[1],
		B:          c[2],
		Brightness: 5,
		Speed:      3,
	}
	if err := f.svc.ApplyLighting(burst); err != nil {
		return
	}
	time.Sleep(f.duration)
	_ = f.svc.ApplyLighting(restore)
}

// flashColor returns the configured color for a direction.
func parseRGB(s string) ([3]uint8, error) {
	var c [3]uint8
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return c, fmt.Errorf("color inválido %q (usá r,g,b)", s)
	}
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < 0 || v > 255 {
			return c, fmt.Errorf("color inválido %q", s)
		}
		c[i] = uint8(v)
	}
	return c, nil
}

// runWatch listens for system events and flashes the keyboard.
func runWatch(f *flasher, notifications bool, notifColor [3]uint8, volume bool, up, down [3]uint8,
	battery bool, threshold int, interval time.Duration, lowColor [3]uint8) error {
	events := make(chan [3]uint8, 8)

	if notifications {
		go watchNotifications(events, notifColor)
	}
	if volume {
		go watchVolume(events, up, down)
	}
	if battery {
		go watchBattery(f, threshold, interval, lowColor)
	}
	if !notifications && !volume && !battery {
		return fmt.Errorf("elegí al menos un evento: --notifications, --volume o --battery")
	}

	// Debounce: burst of events produce a single flash.
	for c := range events {
		select {
		case <-time.After(120 * time.Millisecond):
		}
		// Drain anything queued during the debounce window.
		for len(events) > 0 {
			<-events
		}
		go f.flash(c)
	}
	return nil
}

// watchNotifications follows D-Bus Notify calls.
func watchNotifications(out chan<- [3]uint8, color [3]uint8) {
	cmd := exec.Command("dbus-monitor", "--session",
		"interface='org.freedesktop.Notifications',member='Notify'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if strings.Contains(sc.Text(), "member=Notify") {
			out <- color
		}
	}
}

// watchVolume follows PulseAudio/PipeWire volume changes and flashes green on
// up and red on down.
func watchVolume(out chan<- [3]uint8, up, down [3]uint8) {
	cmd := exec.Command("pactl", "subscribe")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	last := currentVolume()
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "on sink") && !strings.Contains(line, "on sink-input") {
			continue
		}
		v := currentVolume()
		if v == last {
			continue
		}
		if v > last {
			out <- up
		} else {
			out <- down
		}
		last = v
	}
}

// watchBattery polls the battery over 2.4 GHz and alerts when it is low.
func watchBattery(f *flasher, threshold int, interval time.Duration, color [3]uint8) {
	alerted := time.Time{}
	for {
		pct, ok := batteryLevel()
		if ok && pct <= threshold {
			// Re-alert after 10 minutes, or when the level changes.
			if time.Since(alerted) > 10*time.Minute {
				alerted = time.Now()
				notify("Batería del teclado", fmt.Sprintf("Queda %d%%, cargalo", pct))
				f.burst(color, 4)
			}
		} else if ok && pct > threshold {
			alerted = time.Time{}
		}
		time.Sleep(interval)
	}
}

// batteryLevel reads the dongle battery. It fails on USB-C (charging), which
// is fine: we simply skip the check.
func batteryLevel() (int, bool) {
	d, err := dongleOpen()
	if err != nil {
		return 0, false
	}
	defer d.Close()
	pct, err := d.Battery()
	if err != nil {
		return 0, false
	}
	return pct, true
}

// notify sends a desktop notification when notify-send exists.
func notify(title, body string) {
	cmd := exec.Command("notify-send", "-a", "openaula", title, body)
	_ = cmd.Run()
}

// currentVolume returns the default sink volume as a percentage, or -1.
func currentVolume() int {
	if v, ok := volumeFrom("wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@"); ok {
		return v
	}
	if v, ok := volumeFrom("pactl", "get-sink-volume", "@DEFAULT_SINK@"); ok {
		return v
	}
	return -1
}

func volumeFrom(bin string, args ...string) (int, bool) {
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return 0, false
	}
	// wpctl: "Volume: 0.45 [MUTED]" / pactl: "Volume: front-left: 29491 /  45% / ..."
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if strings.HasSuffix(f, "%") {
			v, err := strconv.Atoi(strings.TrimSuffix(f, "%"))
			if err == nil {
				return v, true
			}
		}
		if f == "Volume:" && i+1 < len(fields) {
			if val, err := strconv.ParseFloat(fields[i+1], 64); err == nil {
				return int(val * 100), true
			}
		}
	}
	return 0, false
}
