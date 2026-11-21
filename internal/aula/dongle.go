package aula

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ldgnu/openaula/internal/hidraw"
)

// The 2.4 GHz dongle exposes a 32-byte vendor channel on interface 3.
// Protocol documented by kidush/omarchy-aula-f108pro (MIT), recovered from the
// official Windows driver: every packet is self-contained, carries a checksum
// in the last byte and is answered with an echo of its first three bytes.

// DongleReportSize is the dongle vendor report size (no report ID).
const DongleReportSize = 32

// Dongle command opcodes.
const (
	dongleCmdProbe     = 0x02
	dongleCmdLight     = 0x05
	dongleCmdClock     = 0x0C
	dongleCmdBattery   = 0x20
	dongleCmdRemap     = 0x10
	dongleCmdRemapFN   = 0x12
	dongleCmdPerKey    = 0x14
	dongleCmdResetKeys = 0x1F
	dongleCmdFunc      = 0x07
)

// ErrNoReply is returned when the dongle acknowledges nothing; usually the
// keyboard is asleep.
var ErrNoReply = errors.New("sin respuesta del teclado (¿está dormido? apretá una tecla)")

// Dongle is an open session on the 2.4 GHz vendor channel.
type Dongle struct {
	dev *hidraw.Device
}

// DonglePacket is one 32-byte vendor report.
type DonglePacket [DongleReportSize]byte

// OpenDongle finds and opens the dongle's vendor interface.
func OpenDongle() (*Dongle, error) {
	infos, err := hidraw.Enumerate()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if info.VendorID != DongleVendorID || info.ProductID != DongleProductID {
			continue
		}
		// Interface 3 is the 32-byte in/out vendor channel (usage page 0xFF60).
		if info.Interface != 3 && !(info.UsagePage == 0xFF60 && info.Usage == 0x61) {
			continue
		}
		dev, err := hidraw.Open(info.Path)
		if err != nil {
			return nil, fmt.Errorf("abriendo %s: %w (¿instalaste la regla udev?)", info.Path, err)
		}
		return &Dongle{dev: dev}, nil
	}
	return nil, fmt.Errorf("no encontré el canal vendor del dongle 2.4 GHz")
}

// Close releases the device.
func (d *Dongle) Close() { _ = d.dev.Close() }

func newDonglePacket(cmd, sub, index byte, data ...byte) DonglePacket {
	var p DonglePacket
	p[0], p[1], p[2] = cmd, sub, index
	copy(p[3:DongleReportSize-1], data)
	var sum byte
	for _, b := range p[:DongleReportSize-1] {
		sum += b
	}
	p[DongleReportSize-1] = sum
	return p
}

// exchange writes a packet and waits for the matching reply.
func (d *Dongle) exchange(p DonglePacket) (DonglePacket, error) {
	out := append([]byte{0}, p[:]...) // Leading 0x00 = report ID (none declared).
	buf := make([]byte, 64)
	const readWindow = 250 * time.Millisecond
	for try := 0; try < 2; try++ {
		if _, err := d.dev.Write(out); err != nil {
			return DonglePacket{}, fmt.Errorf("escritura al dongle: %w", err)
		}
		deadline := time.Now().Add(readWindow)
		for {
			left := time.Until(deadline)
			if left <= 0 {
				break
			}
			n, err := d.dev.Read(buf, left)
			if errors.Is(err, hidraw.ErrTimeout) {
				break
			}
			if err != nil {
				return DonglePacket{}, fmt.Errorf("lectura del dongle: %w", err)
			}
			if n >= DongleReportSize && buf[0] == p[0] && buf[1] == p[1] && buf[2] == p[2] {
				var r DonglePacket
				copy(r[:], buf)
				return r, nil
			}
		}
	}
	return DonglePacket{}, ErrNoReply
}

// Probe checks that the keyboard answers behind the dongle.
func (d *Dongle) Probe() error {
	_, err := d.exchange(newDonglePacket(dongleCmdProbe, 0, 0))
	return err
}

// Battery returns the battery percentage (2.4 GHz only).
func (d *Dongle) Battery() (int, error) {
	r, err := d.exchange(newDonglePacket(dongleCmdBattery, 0x01, 0))
	if err != nil {
		return 0, err
	}
	pct := int(r[3])
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}

// SetLighting applies a backlight effect over the dongle.
func (d *Dongle) SetLighting(cfg LightingConfig) error {
	block := lightBlock(cfg)
	_, err := d.exchange(newDonglePacket(dongleCmdLight, 0x10, 0, block...))
	return err
}

// lightBlock is the 16-byte backlight setting shared by both transports.
func lightBlock(cfg LightingConfig) []byte {
	if cfg.Mode == 0 {
		return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xAA, 0x55}
	}
	var colorful byte
	if cfg.Colorful {
		colorful = 1
	}
	return []byte{
		byte(cfg.Mode), cfg.R, cfg.G, cfg.B,
		0, 0, 0, 0,
		colorful, cfg.Brightness, cfg.Speed, cfg.Direction,
		0, 0, 0xAA, 0x55,
	}
}

// SyncClock sets the LCD clock over the dongle.
func (d *Dongle) SyncClock(t time.Time) error {
	data := []byte{
		0x00, 0x01, 0x5A,
		byte(t.Year() % 100), byte(t.Month()), byte(t.Day()),
		byte(t.Hour()), byte(t.Minute()), byte(t.Second()),
		0x00, byte(t.Weekday()),
		0x00, 0x00, 0x00, 0xAA, 0x55,
	}
	_, err := d.exchange(newDonglePacket(dongleCmdClock, 0x10, 0, data...))
	return err
}

// SetPerKeyRGB applies a per-key layout over the dongle.
func (d *Dongle) SetPerKeyRGB(keys []KeyColor, brightness uint8) error {
	if brightness > 5 {
		brightness = 5
	}
	// Header: backlight packet with mode 0x80 and the brightness byte.
	header := make([]byte, 16)
	header[0] = 0x80
	header[8] = brightness
	header[14], header[15] = 0xAA, 0x55
	if _, err := d.exchange(newDonglePacket(dongleCmdLight, 0x10, 0, header...)); err != nil {
		return fmt.Errorf("cabecera per-key: %w", err)
	}

	table := make([]byte, perKeyRGBSize)
	for _, k := range keys {
		off := int(k.LightIndex) * 4
		if off < 0 || off+3 >= perKeyRGBSize-2 {
			continue
		}
		table[off] = k.LightIndex
		table[off+1] = k.R
		table[off+2] = k.G
		table[off+3] = k.B
	}
	table[perKeyRGBSize-2], table[perKeyRGBSize-1] = 0xAA, 0x55
	return d.sendChunks(dongleCmdPerKey, table)
}

// SetKeyRemap sends the normal-layer remap table over the dongle.
func (d *Dongle) SetKeyRemap(remaps []KeyRemap) error {
	return d.sendRemap(dongleCmdRemap, remaps)
}

// SetFnKeyRemap sends the FN-layer remap table over the dongle.
func (d *Dongle) SetFnKeyRemap(remaps []KeyRemap) error {
	return d.sendRemap(dongleCmdRemapFN, remaps)
}

func (d *Dongle) sendRemap(cmd byte, remaps []KeyRemap) error {
	table := make([]byte, remapBufSize)
	for _, r := range remaps {
		off := int(r.SourceIndex) * 4
		if off <= 0 || off+3 >= remapBufSize-2 {
			continue
		}
		table[off] = byte(r.Action)
		table[off+1] = r.Param1
		table[off+2] = r.Param2
		table[off+3] = r.Param3
	}
	table[remapBufSize-2], table[remapBufSize-1] = 0xAA, 0x55
	return d.sendChunks(cmd, table)
}

// ResetFnKeyRemap clears the FN layer (the dongle only exposes a global reset,
// so both layers are cleared by the firmware).
func (d *Dongle) ResetFnKeyRemap() error { return d.ResetKeyRemap() }

// ResetKeyRemap asks the keyboard to clear its key mapping.
func (d *Dongle) ResetKeyRemap() error {
	_, err := d.exchange(newDonglePacket(dongleCmdResetKeys, 0, 0))
	return err
}

// sendChunks splits data into 28-byte chunks with 2 ms between packets.
func (d *Dongle) sendChunks(cmd byte, data []byte) error {
	const chunk = 28
	for i, idx := 0, byte(0); i < len(data); i, idx = i+chunk, idx+1 {
		end := i + chunk
		if end > len(data) {
			end = len(data)
		}
		if _, err := d.exchange(newDonglePacket(cmd, 0x1C, idx, data[i:end]...)); err != nil {
			return fmt.Errorf("chunk %d: %w", idx, err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}

// UploadLCD uploads a raw LCD buffer (header + RGB565 frames + AA 55) over the
// dongle. The vendor hides this in wireless mode, so treat it as experimental.
// progress is called with (chunksSent, totalChunks).
func (d *Dongle) UploadLCD(buf []byte, slot uint8, progress func(sent, total int)) error {
	const chunk = 28
	total := len(buf)
	chunks := (total + chunk - 1) / chunk

	header := newDonglePacket(0x7F, 0x03, 0, slot,
		byte(chunks), byte(chunks>>8), byte(chunks>>16))
	if _, err := d.exchange(header); err != nil {
		return fmt.Errorf("cabecera LCD: %w", err)
	}

	for i := 0; i < chunks; i++ {
		start := i * chunk
		end := start + chunk
		if end > total {
			end = total
		}
		p := newDonglePacket(0x80|byte(i>>16), byte(i), byte(i>>8), buf[start:end]...)
		if _, err := d.exchange(p); err != nil {
			return fmt.Errorf("chunk %d/%d: %w", i+1, chunks, err)
		}
		if progress != nil && (i%64 == 0 || i == chunks-1) {
			progress(i+1, chunks)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}

// FirmwareFromBCD converts HID bcdDevice to the firmware number the vendor UI
// shows (0x0120 -> 120).
func FirmwareFromBCD(bcd uint16) string {
	s := fmt.Sprintf("%x", bcd)
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}
