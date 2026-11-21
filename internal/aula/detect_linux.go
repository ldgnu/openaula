//go:build linux

package aula

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// detectPlatform reads the USB descriptors from sysfs.
func detectPlatform(vid, pid uint16) (Info, bool) {
	entries, err := filepath.Glob("/sys/bus/usb/devices/*")
	if err != nil {
		return Info{}, false
	}
	for _, dir := range entries {
		v, err1 := readHex(filepath.Join(dir, "idVendor"))
		p, err2 := readHex(filepath.Join(dir, "idProduct"))
		if err1 != nil || err2 != nil {
			continue
		}
		if v != vid || p != pid {
			continue
		}
		info := Info{
			Connected: true,
			Transport: ConnUSB,
			VID:       v,
			PID:       p,
		}
		if b, err := readHex(filepath.Join(dir, "bcdDevice")); err == nil {
			info.BCDDevice = b
		}
		info.Product = readString(filepath.Join(dir, "product"))
		info.Manufacturer = readString(filepath.Join(dir, "manufacturer"))
		info.Serial = readString(filepath.Join(dir, "serial"))
		info.NumInterfaces = len(glob(filepath.Join(dir, "*:1.*")))
		return info, true
	}
	return Info{}, false
}

// detectDongle reports the 2.4 GHz dongle when no wired keyboard is present.
func detectDongle() (Info, bool) {
	info, ok := detectPlatform(DongleVendorID, DongleProductID)
	if !ok {
		return Info{}, false
	}
	info.Transport = Conn24G
	if info.Product == "" {
		info.Product = "F108Pro Dongle"
	}
	return info, true
}

func readHex(path string) (uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 16, 16)
	return uint16(v), err
}

func readString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func glob(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	return m
}
