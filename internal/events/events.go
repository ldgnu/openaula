// Package events reacts to system events by flashing the keyboard.
package events

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/device"
	"github.com/ldgnu/openaula/internal/state"
)

// Opts configures which events are watched.
type Opts struct {
	Notifications bool
	NotifColor    [3]uint8

	Volume bool
	Up     [3]uint8
	Down   [3]uint8

	Battery   bool
	Threshold int
	LowColor  [3]uint8

	Sleep bool

	Lock      bool
	LockColor [3]uint8

	FullCharge bool
	FullLevel  int
	FullColor  [3]uint8

	LEDs bool

	Schedule   bool
	DayStart   int
	DayEnd     int
	DayColor   [3]uint8
	NightColor [3]uint8

	Duration time.Duration
	Interval time.Duration
}

// Any reports whether at least one source is enabled.
func (o Opts) Any() bool {
	return o.Notifications || o.Volume || o.Battery || o.Sleep || o.Lock ||
		o.FullCharge || o.LEDs || o.Schedule
}

// Run watches the selected events until ctx is cancelled.
func Run(ctx context.Context, svc *device.Service, store *state.Store, o Opts) error {
	if !o.Any() {
		return fmt.Errorf("elegí al menos un evento")
	}
	if o.Duration <= 0 {
		o.Duration = 150 * time.Millisecond
	}
	if o.Interval <= 0 {
		o.Interval = time.Minute
	}

	f := &flasher{svc: svc, store: store, duration: o.Duration}
	events := make(chan [3]uint8, 8)

	if o.Notifications {
		go watchNotifications(ctx, events, o.NotifColor)
	}
	if o.Volume {
		go watchVolume(ctx, events, o.Up, o.Down)
	}
	if o.Battery {
		go watchBattery(ctx, f, o.Threshold, o.Interval, o.LowColor)
	}
	if o.FullCharge {
		go watchFullCharge(ctx, f, o.FullLevel, o.FullColor, o.Interval)
	}
	if o.Sleep || o.Lock {
		ov := &overlay{svc: svc}
		if o.Sleep {
			go watchSleep(ctx, f, ov)
		}
		if o.Lock {
			go watchLock(ctx, f, ov, o.LockColor)
		}
	}
	if o.LEDs {
		go watchLEDs(ctx, f)
	}
	if o.Schedule {
		go watchSchedule(ctx, f, o)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case c := <-events:
			// Debounce a burst into a single flash.
			select {
			case <-time.After(120 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
			for len(events) > 0 {
				<-events
			}
			go f.flash(c)
		}
	}
}

// Flash applies a short lighting burst and restores the saved configuration.
func Flash(svc *device.Service, store *state.Store, c [3]uint8, d time.Duration) {
	if d <= 0 {
		d = 150 * time.Millisecond
	}
	f := &flasher{svc: svc, store: store, duration: d}
	f.flash(c)
}

// ---- flasher ----

type flasher struct {
	svc   *device.Service
	store *state.Store

	mu       sync.Mutex
	duration time.Duration
}

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
	burst := state.Lighting{Mode: 1, R: c[0], G: c[1], B: c[2], Brightness: 5, Speed: 3}
	if err := f.svc.ApplyLighting(burst); err != nil {
		return
	}
	time.Sleep(f.duration)
	_ = f.svc.ApplyLighting(restore)
}

// ---- sources ----

func watchNotifications(ctx context.Context, out chan<- [3]uint8, color [3]uint8) {
	cmd := exec.CommandContext(ctx, "dbus-monitor", "--session",
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
			select {
			case out <- color:
			case <-ctx.Done():
				return
			}
		}
	}
}

func watchVolume(ctx context.Context, out chan<- [3]uint8, up, down [3]uint8) {
	cmd := exec.CommandContext(ctx, "pactl", "subscribe")
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
		c := down
		if v > last {
			c = up
		}
		select {
		case out <- c:
		case <-ctx.Done():
			return
		}
		last = v
	}
}

func watchBattery(ctx context.Context, f *flasher, threshold int, interval time.Duration, color [3]uint8) {
	alerted := time.Time{}
	for {
		pct, ok := batteryLevel()
		if ok && pct <= threshold {
			if time.Since(alerted) > 10*time.Minute {
				alerted = time.Now()
				Notify("Batería del teclado", fmt.Sprintf("Queda %d%%, cargalo", pct))
				f.burst(color, 4)
			}
		} else if ok {
			alerted = time.Time{}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func watchFullCharge(ctx context.Context, f *flasher, level int, color [3]uint8, interval time.Duration) {
	wasBelow := true
	for {
		if pct, ok := batteryLevel(); ok {
			if pct < level {
				wasBelow = true
			} else if wasBelow {
				wasBelow = false
				Notify("Teclado cargado", fmt.Sprintf("Batería al %d%%", pct))
				f.burst(color, 3)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// ---- lock and suspend ----

type overlay struct {
	mu    sync.Mutex
	svc   *device.Service
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
	if o.saved == nil {
		return
	}
	_ = o.svc.ApplyLighting(*o.saved)
	o.saved = nil
}

func watchSleep(ctx context.Context, f *flasher, o *overlay) {
	cmd := exec.CommandContext(ctx, "dbus-monitor", "--system",
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

func watchLock(ctx context.Context, f *flasher, o *overlay, color [3]uint8) {
	cmd := exec.CommandContext(ctx, "dbus-monitor", "--session",
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
				Mode: 1, R: color[0], G: color[1], B: color[2], Brightness: 2, Speed: 3,
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

// ---- LEDs ----

func watchLEDs(ctx context.Context, f *flasher) {
	var wg sync.WaitGroup
	for _, dev := range keyboardEventDevices() {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			readLEDs(ctx, path, f)
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

func readLEDs(ctx context.Context, path string, f *flasher) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	buf := make([]byte, 24) // struct input_event on 64-bit.
	lastCaps, lastNum := -1, -1
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
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

// ---- schedule ----

func watchSchedule(ctx context.Context, f *flasher, o Opts) {
	current := -1
	for {
		hour := time.Now().Hour()
		phase := 1
		if hour >= o.DayStart && hour < o.DayEnd {
			phase = 0
		}
		if phase != current {
			current = phase
			color := o.DayColor
			brightness := uint8(5)
			if phase == 1 {
				color = o.NightColor
				brightness = 2
			}
			if st, err := f.store.Load(); err == nil {
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
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

// ---- helpers ----

// ParseRGB parses "r,g,b".
func ParseRGB(s string) ([3]uint8, error) {
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

// Notify sends a desktop notification when notify-send exists.
func Notify(title, body string) {
	_ = exec.Command("notify-send", "-a", "openaula", title, body).Run()
}

func batteryLevel() (int, bool) {
	d, err := aula.OpenDongle()
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
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if strings.HasSuffix(f, "%") {
			if v, err := strconv.Atoi(strings.TrimSuffix(f, "%")); err == nil {
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
