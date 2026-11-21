package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ldgnu/openaula/internal/state"
)

// watchOpts configures the event watcher.
type watchOpts struct {
	notifications bool
	notifColor    [3]uint8

	volume bool
	up     [3]uint8
	down   [3]uint8

	battery   bool
	threshold int
	interval  time.Duration
	lowColor  [3]uint8

	sleep bool

	lock      bool
	lockColor [3]uint8

	fullCharge bool
	fullLevel  int
	fullColor  [3]uint8

	leds bool

	schedule   bool
	dayStart   int
	dayEnd     int
	dayColor   [3]uint8
	nightColor [3]uint8
}

// overlay remembers the lighting before lock or suspend so it can be restored.
type overlay struct {
	mu    sync.Mutex
	svc   interface{ ApplyLighting(state.Lighting) error }
	saved *state.Lighting
}

func (o *overlay) stash(l state.Lighting) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.saved = &l
}

func (o *overlay) restore() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.saved == nil || o.svc == nil {
		return
	}
	_ = o.svc.ApplyLighting(*o.saved)
	o.saved = nil
}

// watchSleep turns the backlight off when the machine suspends and restores it
// on resume.
func watchSleep(f *flasher, o *overlay) {
	cmd := exec.Command("dbus-monitor", "--system",
		"interface='org.freedesktop.login1.Manager',member='PrepareForSleep'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	sc := bufio.NewScanner(stdout)
	armed := false
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "boolean") {
			continue
		}
		sleeping := strings.Contains(line, "true")
		if sleeping && !armed {
			armed = true
			if st, err := f.store.Load(); err == nil {
				o.stash(st.Lighting)
			}
			_ = f.svc.ApplyLighting(state.Lighting{Mode: 0})
		} else if !sleeping && armed {
			armed = false
			o.restore()
		}
	}
}

// watchLock dims the keyboard while the session is locked.
func watchLock(f *flasher, o *overlay, color [3]uint8) {
	cmd := exec.Command("dbus-monitor", "--session",
		"interface='org.freedesktop.login1.Session',member='Lock'",
		"interface='org.freedesktop.login1.Session',member='Unlock'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	sc := bufio.NewScanner(stdout)
	locked := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, "member=Lock"):
			if locked {
				continue
			}
			locked = true
			if st, err := f.store.Load(); err == nil {
				o.stash(st.Lighting)
			}
			_ = f.svc.ApplyLighting(state.Lighting{
				Mode: 1, R: color[0], G: color[1], B: color[2],
				Brightness: 2, Speed: 3,
			})
		case strings.Contains(line, "member=Unlock"):
			if !locked {
				continue
			}
			locked = false
			o.restore()
		}
	}
}

// watchLEDs flashes on Caps Lock / Num Lock changes read from evdev.
func watchLEDs(f *flasher) {
	devices := keyboardEventDevices()
	if len(devices) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, dev := range devices {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			readLEDs(path, f)
		}(dev)
	}
	wg.Wait()
}

func keyboardEventDevices() []string {
	entries, err := filepath.Glob("/sys/class/input/event*/device/name")
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(data))
		if !strings.Contains(name, "F108Pro") && !strings.Contains(name, "AULA") {
			continue
		}
		event := strings.TrimSuffix(strings.TrimPrefix(filepath.Dir(entry), "/sys/class/input/"), "/device")
		out = append(out, "/dev/input/"+event)
	}
	return out
}

// readLEDs parses input_event records and reacts to LED changes.
func readLEDs(path string, f *flasher) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	const eventSize = 24 // struct input_event on 64-bit.
	buf := make([]byte, eventSize)
	lastCaps, lastNum := -1, -1
	for {
		if _, err := io.ReadFull(file, buf); err != nil {
			return
		}
		typ := binary.LittleEndian.Uint16(buf[16:18])
		code := binary.LittleEndian.Uint16(buf[18:20])
		value := int32(binary.LittleEndian.Uint32(buf[20:24]))
		if typ != 0x11 { // EV_LED
			continue
		}
		switch code {
		case 0x01: // LED_CAPSL
			if lastCaps != -1 && int(value) != lastCaps {
				f.flash([3]uint8{255, 170, 0})
			}
			lastCaps = int(value)
		case 0x00: // LED_NUML
			if lastNum != -1 && int(value) != lastNum {
				f.flash([3]uint8{0, 170, 255})
			}
			lastNum = int(value)
		}
	}
}

// watchSchedule switches between a day and a night color every minute.
func watchSchedule(f *flasher, o *watchOpts) {
	current := -1 // 0 day, 1 night
	for {
		hour := time.Now().Hour()
		phase := 1
		if hour >= o.dayStart && hour < o.dayEnd {
			phase = 0
		}
		if phase != current {
			current = phase
			color := o.dayColor
			brightness := uint8(5)
			if phase == 1 {
				color = o.nightColor
				brightness = 2
			}
			st, err := f.store.Load()
			if err == nil {
				l := st.Lighting
				if l.Mode == 0 {
					l.Mode = 1
				}
				l.R, l.G, l.B = color[0], color[1], color[2]
				l.Brightness = brightness
				if err := f.svc.ApplyLighting(l); err == nil {
					st.Lighting = l
					_ = f.store.Save(st)
				}
			}
		}
		time.Sleep(time.Minute)
	}
}

// watchFullCharge alerts once when the battery reaches the configured level.
func watchFullCharge(f *flasher, level int, color [3]uint8, interval time.Duration) {
	wasBelow := true
	for {
		pct, ok := batteryLevel()
		if ok {
			if pct < level {
				wasBelow = true
			} else if wasBelow {
				wasBelow = false
				notify("Teclado cargado", fmt.Sprintf("Batería al %d%%", pct))
				f.burst(color, 3)
			}
		}
		time.Sleep(interval)
	}
}
