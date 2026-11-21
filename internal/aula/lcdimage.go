package aula

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"io"
)

// LCD screen geometry and buffer layout.
const (
	ScreenWidth   = 240
	ScreenHeight  = 135
	frameBytes    = ScreenWidth * ScreenHeight * 2 // RGB565, 64800 bytes.
	gifHeaderSize = 256
	// MaxFrames is the safe limit; larger uploads corrupted the built-in
	// menu graphics on the reference keyboard.
	MaxFrames = 141
)

// RGB888ToRGB565 converts an 8-bit RGB triplet to RGB565.
func RGB888ToRGB565(r, g, b uint8) uint16 {
	return (uint16(r>>3) << 11) | (uint16(g>>2) << 5) | uint16(b>>3)
}

// BuildSolidLCDImage builds a one-frame LCD buffer filled with a solid color.
func BuildSolidLCDImage(r, g, b uint8) []byte {
	px := RGB888ToRGB565(r, g, b)
	buf := buildHeader(1, []int{1})
	offset := gifHeaderSize
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		binary.LittleEndian.PutUint16(buf[offset:], px)
		offset += 2
	}
	return padToPages(buf)
}

// BuildGIFLCDImage decodes a GIF and builds the raw buffer the keyboard expects.
// Frames are composited onto a canvas and converted to RGB565.
func BuildGIFLCDImage(r io.Reader) ([]byte, int, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return nil, 0, fmt.Errorf("decoding gif: %w", err)
	}
	if len(g.Image) == 0 {
		return nil, 0, fmt.Errorf("gif has no frames")
	}

	frames := len(g.Image)
	if frames > MaxFrames {
		return nil, 0, fmt.Errorf("gif has %d frames, the safe limit is %d", frames, MaxFrames)
	}

	delays := make([]int, frames)
	for i := range delays {
		// GIF delays are centiseconds; the keyboard stores half of that.
		d := 1
		if i < len(g.Delay) {
			d = g.Delay[i] / 2
		}
		if d < 1 {
			d = 1
		}
		if d > 255 {
			d = 255
		}
		delays[i] = d
	}

	buf := buildHeader(frames, delays)

	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	offset := gifHeaderSize
	for i := 0; i < frames; i++ {
		frame := g.Image[i]
		b := frame.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := frame.At(x, y)
				if _, _, _, a := c.RGBA(); a == 0 {
					continue
				}
				canvas.Set(x, y, c)
			}
		}

		for y := 0; y < ScreenHeight; y++ {
			for x := 0; x < ScreenWidth; x++ {
				var r8, g8, b8 uint8
				if x < g.Config.Width && y < g.Config.Height {
					rr, gg, bb, _ := canvas.At(x, y).RGBA()
					r8, g8, b8 = uint8(rr>>8), uint8(gg>>8), uint8(bb>>8)
				}
				binary.LittleEndian.PutUint16(buf[offset:], RGB888ToRGB565(r8, g8, b8))
				offset += 2
			}
		}

		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalBackground, gif.DisposalPrevious:
				for y := b.Min.Y; y < b.Max.Y; y++ {
					for x := b.Min.X; x < b.Max.X; x++ {
						canvas.Set(x, y, image.Black)
					}
				}
			}
		}
	}

	return padToPages(buf), frames, nil
}

func buildHeader(frames int, delays []int) []byte {
	buf := make([]byte, gifHeaderSize+frameBytes*frames)
	for i := range buf {
		buf[i] = 0xFF
	}
	buf[0] = byte(frames)
	for i, d := range delays {
		buf[1+i] = byte(d)
	}
	return buf
}

func padToPages(buf []byte) []byte {
	if len(buf)%lcdPageSize == 0 {
		return buf
	}
	padded := make([]byte, (len(buf)/lcdPageSize+1)*lcdPageSize)
	copy(padded, buf)
	for i := len(buf); i < len(padded); i++ {
		padded[i] = 0xFF
	}
	return padded
}
