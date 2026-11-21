package aula

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"io"

	"golang.org/x/image/draw"
	_ "image/jpeg"
	_ "image/png"
)

// BuildAnyLCDImage builds an LCD buffer from a GIF, PNG or JPEG.
//
// Static images are scaled to fit the 240x135 screen preserving aspect ratio
// and letterboxed on black. Animated GIFs keep their frames and delays.
// The return value is the number of frames.
func BuildAnyLCDImage(r io.Reader) ([]byte, int, error) {
	data, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		return nil, 0, err
	}
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("archivo vacío")
	}

	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return BuildGIFLCDImage(bytes.NewReader(data))
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("formato de imagen no soportado (usá GIF, PNG o JPEG): %w", err)
	}

	buf := buildHeader(1, []int{100}) // 2 s per frame (delay units are 20 ms).
	canvas := fitToScreen(img)
	offset := gifHeaderSize
	for y := 0; y < ScreenHeight; y++ {
		for x := 0; x < ScreenWidth; x++ {
			rr, gg, bb, _ := canvas.At(x, y).RGBA()
			binary.LittleEndian.PutUint16(buf[offset:], RGB888ToRGB565(uint8(rr>>8), uint8(gg>>8), uint8(bb>>8)))
			offset += 2
		}
	}
	return padToPages(buf), 1, nil
}

// fitToScreen scales src to fit ScreenWidth x ScreenHeight and centers it.
func fitToScreen(src image.Image) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	// Black background for letterboxing.
	for i := 3; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 0xFF
	}

	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	if sw == 0 || sh == 0 {
		return dst
	}
	scale := min(float64(ScreenWidth)/sw, float64(ScreenHeight)/sh)
	w := int(sw * scale)
	h := int(sh * scale)
	rect := image.Rect((ScreenWidth-w)/2, (ScreenHeight-h)/2, (ScreenWidth+w)/2, (ScreenHeight+h)/2)
	draw.ApproxBiLinear.Scale(dst, rect, src, b, draw.Over, nil)
	return dst
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// FrameCount reports how many frames an LCD buffer contains.
func FrameCount(buf []byte) int {
	if len(buf) == 0 {
		return 0
	}
	return int(buf[0])
}

var _ = gif.DecodeAll
