package aula

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

func TestBuildSolidSizes(t *testing.T) {
	buf := BuildSolidLCDImage(10, 20, 30)
	if len(buf)%lcdPageSize != 0 {
		t.Fatalf("buffer %d is not page aligned", len(buf))
	}
	if FrameCount(buf) != 1 {
		t.Fatalf("frames = %d, want 1", FrameCount(buf))
	}
	if len(buf) < gifHeaderSize+frameBytes {
		t.Fatalf("buffer too small: %d", len(buf))
	}
}

func TestBuildPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	buf, frames, err := BuildAnyLCDImage(bytes.NewReader(raw.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if frames != 1 {
		t.Fatalf("frames = %d, want 1", frames)
	}
	if len(buf)%lcdPageSize != 0 {
		t.Fatalf("not page aligned: %d", len(buf))
	}
	// Letterboxed 300x300 into 240x135 leaves black bars top and bottom.
	if px := buf[gifHeaderSize:]; px[0] != 0 || px[1] != 0 {
		t.Fatalf("expected black top-left pixel, got %v", px[:2])
	}
}

func TestBuildGIFTooManyFrames(t *testing.T) {
	g := &gif.GIF{}
	for i := 0; i < MaxFrames+1; i++ {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 240, 135), color.Palette{color.Black, color.White}))
		g.Delay = append(g.Delay, 10)
	}
	var raw bytes.Buffer
	if err := gif.EncodeAll(&raw, g); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildAnyLCDImage(bytes.NewReader(raw.Bytes())); err == nil {
		t.Fatal("expected error for too many frames")
	}
}
