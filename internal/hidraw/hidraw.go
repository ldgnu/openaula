// Package hidraw discovers and talks to HID devices through Linux /dev/hidraw.
//
// Based on the same approach used by kidush/omarchy-aula-f108pro (MIT).
package hidraw

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Info describes one hidraw node.
type Info struct {
	Path      string
	Name      string
	VendorID  uint16
	ProductID uint16
	Interface int
	UsagePage uint16
	Usage     uint16
}

// Enumerate lists all hidraw nodes.
func Enumerate() ([]Info, error) {
	nodes, err := filepath.Glob("/sys/class/hidraw/hidraw*")
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, n := range nodes {
		if info, err := readInfo(n); err == nil {
			out = append(out, info)
		}
	}
	return out, nil
}

func readInfo(sysPath string) (Info, error) {
	info := Info{Path: "/dev/" + filepath.Base(sysPath), Interface: -1}
	uevent, err := os.ReadFile(filepath.Join(sysPath, "device", "uevent"))
	if err != nil {
		return info, err
	}
	for _, line := range strings.Split(string(uevent), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "HID_NAME":
			info.Name = v
		case "HID_ID":
			parts := strings.Split(v, ":")
			if len(parts) == 3 {
				vid, _ := strconv.ParseUint(parts[1], 16, 32)
				pid, _ := strconv.ParseUint(parts[2], 16, 32)
				info.VendorID, info.ProductID = uint16(vid), uint16(pid)
			}
		}
	}
	if dev, err := filepath.EvalSymlinks(filepath.Join(sysPath, "device")); err == nil {
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(dev), "bInterfaceNumber")); err == nil {
			if n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 16, 8); err == nil {
				info.Interface = int(n)
			}
		}
	}
	if desc, err := os.ReadFile(filepath.Join(sysPath, "device", "report_descriptor")); err == nil {
		info.UsagePage, info.Usage = firstUsage(desc)
	}
	return info, nil
}

func firstUsage(d []byte) (page, usage uint16) {
	var gotPage, gotUsage bool
	for i := 0; i < len(d) && !(gotPage && gotUsage); {
		prefix := d[i]
		size := int(prefix & 0x03)
		if size == 3 {
			size = 4
		}
		if i+1+size > len(d) {
			break
		}
		var val uint32
		for j := 0; j < size; j++ {
			val |= uint32(d[i+1+j]) << (8 * j)
		}
		switch prefix & 0xFC {
		case 0x04:
			if !gotPage {
				page, gotPage = uint16(val), true
			}
		case 0x08:
			if !gotUsage {
				usage, gotUsage = uint16(val), true
			}
		}
		i += 1 + size
	}
	return page, usage
}

// Device is an open hidraw node.
type Device struct {
	f *os.File
}

// Open opens a hidraw node for reading and writing.
func Open(path string) (*Device, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Device{f: f}, nil
}

// Close releases the node.
func (d *Device) Close() error { return d.f.Close() }

// Write sends an output report (leading 0x00 report ID for devices without IDs).
func (d *Device) Write(report []byte) (int, error) { return d.f.Write(report) }

// SetFeature sends a feature report with HIDIOCSFEATURE.
func (d *Device) SetFeature(report []byte) error { return d.ioctl(0x06, report) }

// GetFeature reads a feature report with HIDIOCGFEATURE.
func (d *Device) GetFeature(report []byte) error { return d.ioctl(0x07, report) }

func (d *Device) ioctl(nr uintptr, buf []byte) error {
	if len(buf) == 0 {
		return errors.New("hidraw: buffer vacío")
	}
	req := uintptr(3)<<30 | uintptr(len(buf))<<16 | uintptr('H')<<8 | nr
	for {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, d.f.Fd(), req, uintptr(unsafe.Pointer(&buf[0])))
		if errno == unix.EINTR {
			continue
		}
		if errno != 0 {
			return fmt.Errorf("hidraw ioctl: %w", errno)
		}
		return nil
	}
}

// ErrTimeout is returned by Read when no report arrives in time.
var ErrTimeout = errors.New("hidraw: read timeout")

// Read waits up to timeout for one input report.
func (d *Device) Read(buf []byte, timeout time.Duration) (int, error) {
	fds := []unix.PollFd{{Fd: int32(d.f.Fd()), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("hidraw poll: %w", err)
		}
		if n == 0 {
			return 0, ErrTimeout
		}
		return d.f.Read(buf)
	}
}

var _ = unsafe.Pointer(nil)
