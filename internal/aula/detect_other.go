//go:build !linux

package aula

// detectPlatform is implemented through the HID API on Windows and is not
// wired to sysfs elsewhere.
func detectPlatform(vid, pid uint16) (Info, bool) { return Info{}, false }

func detectDongle() (Info, bool) { return Info{}, false }
