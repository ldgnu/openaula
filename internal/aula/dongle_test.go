package aula

import (
	"testing"
	"time"
)

func TestDonglePacketChecksum(t *testing.T) {
	p := newDonglePacket(dongleCmdLight, 0x10, 0, lightBlock(LightingConfig{
		Mode: ModeStatic, R: 255, Brightness: 5, Speed: 3,
	})...)
	var sum byte
	for _, b := range p[:DongleReportSize-1] {
		sum += b
	}
	if p[DongleReportSize-1] != sum {
		t.Fatalf("checksum = %#x, want %#x", p[DongleReportSize-1], sum)
	}
	// Documented example: static red, brightness 5, speed 3 -> 05 10 00 01 FF ...
	if p[0] != 0x05 || p[1] != 0x10 || p[3] != 0x01 || p[4] != 0xFF {
		t.Fatalf("unexpected header: % x", p[:8])
	}
	if p[3+14] != 0xAA || p[3+15] != 0x55 {
		t.Fatalf("missing AA 55 trailer: % x", p[:24])
	}
}

func TestLightBlockOff(t *testing.T) {
	block := lightBlock(LightingConfig{Mode: 0})
	if len(block) != 16 {
		t.Fatalf("len = %d", len(block))
	}
	for i := 0; i < 14; i++ {
		if block[i] != 0 {
			t.Fatalf("byte %d = %#x, want 0", i, block[i])
		}
	}
	if block[14] != 0xAA || block[15] != 0x55 {
		t.Fatalf("bad trailer: % x", block[14:])
	}
}

func TestClockPacket(t *testing.T) {
	d := &Dongle{}
	_ = d
	tm := time.Date(2026, 9, 29, 14, 30, 5, 0, time.Local)
	fields := []byte{
		0x00, 0x01, 0x5A,
		byte(tm.Year() % 100), byte(tm.Month()), byte(tm.Day()),
		byte(tm.Hour()), byte(tm.Minute()), byte(tm.Second()),
		0x00, byte(tm.Weekday()),
	}
	if fields[3] != 26 || fields[4] != 9 || fields[5] != 29 {
		t.Fatalf("bad date fields: % x", fields)
	}
}

func TestFirmwareFromBCD(t *testing.T) {
	cases := map[uint16]string{0x0138: "138", 0x0120: "120", 0x0000: "0"}
	for in, want := range cases {
		if got := FirmwareFromBCD(in); got != want {
			t.Fatalf("FirmwareFromBCD(%#x) = %q, want %q", in, got, want)
		}
	}
}
