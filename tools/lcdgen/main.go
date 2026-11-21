package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"log"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	W, H = 240, 135
	N    = 8
)

func loadFont(path string, size float64) font.Face {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		log.Fatal(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		log.Fatal(err)
	}
	return face
}

func drawCenter(dst *image.RGBA, text string, face font.Face, y int, c color.RGBA) {
	w := font.MeasureString(face, text).Round()
	dr := &font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face,
		Dot: fixed.Point26_6{X: fixed.I((W - w) / 2), Y: fixed.I(y)}}
	dr.DrawString(text)
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// boxBlur applies a separable box blur with the given radius.
func boxBlur(src []float64, w, h, r int) []float64 {
	tmp := make([]float64, len(src))
	out := make([]float64, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			var n int
			for k := -r; k <= r; k++ {
				xx := x + k
				if xx < 0 || xx >= w {
					continue
				}
				sum += src[y*w+xx]
				n++
			}
			tmp[y*w+x] = sum / float64(n)
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			var n int
			for k := -r; k <= r; k++ {
				yy := y + k
				if yy < 0 || yy >= h {
					continue
				}
				sum += tmp[yy*w+x]
				n++
			}
			out[y*w+x] = sum / float64(n)
		}
	}
	return out
}

func main() {
	logoPath := flag.String("logo", "", "PNG con el logo a animar")
	outPath := flag.String("out", "animation.gif", "GIF de salida")
	tag := flag.String("tag", "LDGNU", "texto grande de la primera mitad")
	sub := flag.String("sub", "Lawless Disorder", "texto chico de la primera mitad")
	line1 := flag.String("line1", "if it moves,", "primera línea de la segunda mitad")
	line2 := flag.String("line2", "compile it", "segunda línea de la segunda mitad")
	count := flag.Int("frames", 8, "cantidad de cuadros")
	delayMs := flag.Int("delay", 750, "milisegundos por cuadro")
	flag.Parse()

	if *logoPath == "" {
		log.Fatal("falta -logo")
	}
	lf, err := os.Open(*logoPath)
	if err != nil {
		log.Fatal(err)
	}
	src, err := png.Decode(lf)
	if err != nil {
		log.Fatal(err)
	}
	lf.Close()

	// Escalar el logo a 52 px de alto.
	sh := 52
	sw := int(float64(src.Bounds().Dx()) * float64(sh) / float64(src.Bounds().Dy()))
	logo := image.NewRGBA(image.Rect(0, 0, sw, sh))
	xdraw.ApproxBiLinear.Scale(logo, logo.Bounds(), src, src.Bounds(), draw.Over, nil)

	big := loadFont("/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf", 17)
	mid := loadFont("/usr/share/fonts/dejavu/DejaVuSans.ttf", 12)
	small := loadFont("/usr/share/fonts/dejavu/DejaVuSans.ttf", 9)

	var imgs []*image.Paletted
	var delays []int

	cx, cy := W/2, 46
	for i := 0; i < *count; i++ {
		ph := 2 * math.Pi * float64(i) / float64(*count)
		pulse := 0.6 + 0.4*(0.5-0.5*math.Cos(ph))

		img := image.NewRGBA(image.Rect(0, 0, W, H))
		bg := color.RGBA{10, 8, 16, 255}
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				img.SetRGBA(x, y, bg)
			}
		}

		// Glow suave: máscara elíptica difuminada.
		mask := make([]float64, W*H)
		rx := float64(sw/2 + 34)
		ry := rx * 0.62
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				dx := (float64(x) - float64(cx)) / rx
				dy := (float64(y) - float64(cy)) / ry
				d := math.Sqrt(dx*dx + dy*dy)
				if d < 1 {
					mask[y*W+x] = math.Pow(1-d, 2)
				}
			}
		}
		mask = boxBlur(mask, W, H, 6)
		mask = boxBlur(mask, W, H, 6)
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				m := mask[y*W+x] * pulse
				if m <= 0.003 {
					continue
				}
				c := img.RGBAAt(x, y)
				img.SetRGBA(x, y, color.RGBA{
					clamp8(float64(c.R) + 150*m*0.55),
					clamp8(float64(c.G) + 90*m*0.55),
					clamp8(float64(c.B) + 220*m*0.55),
					255,
				})
			}
		}

		// Logo con pulso.
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				c := logo.RGBAAt(x, y)
				if c.A == 0 {
					continue
				}
				px := cx - sw/2 + x
				py := cy - sh/2 + y
				if px < 0 || py < 0 || px >= W || py >= H {
					continue
				}
				img.SetRGBA(px, py, color.RGBA{
					uint8(float64(c.R) * pulse),
					uint8(float64(c.G) * pulse),
					uint8(float64(c.B) * pulse),
					c.A,
				})
			}
		}

		half := *count / 2
		if i < half {
			a := float64(i+1) / float64(half)
			drawCenter(img, *tag, big, 92, color.RGBA{uint8(200 * a), uint8(190 * a), uint8(255 * a), 255})
			drawCenter(img, *sub, small, 108, color.RGBA{uint8(150 * a), uint8(150 * a), uint8(170 * a), 255})
		} else {
			a := float64(i-half+1) / float64(half)
			drawCenter(img, *line1, mid, 92, color.RGBA{uint8(230 * a), uint8(225 * a), uint8(255 * a), 255})
			drawCenter(img, *line2, mid, 106, color.RGBA{uint8(230 * a), uint8(225 * a), uint8(255 * a), 255})
		}

		pal := image.NewPaletted(img.Bounds(), append(color.Palette{color.RGBA{0, 0, 0, 255}}, palette128(img)...))
		draw.FloydSteinberg.Draw(pal, img.Bounds(), img, image.Point{})
		imgs = append(imgs, pal)
		delays = append(delays, *delayMs/10)
	}

	out, err := os.Create(*outPath)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := gif.EncodeAll(out, &gif.GIF{Image: imgs, Delay: delays, LoopCount: 0}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ok %d frames, %d ms por cuadro -> %s\n", len(imgs), delays[0]*10, *outPath)
}

// palette128 builds a 6x6x6-ish palette plus greys.
func palette128(img *image.RGBA) color.Palette {
	var p color.Palette
	// 96 grises.
	for i := 0; i < 96; i++ {
		v := uint8(i * 255 / 95)
		p = append(p, color.RGBA{v, v, v, 255})
	}
	// 96 tonos del glow violeta.
	for i := 0; i < 96; i++ {
		t := float64(i) / 95
		p = append(p, color.RGBA{
			uint8(20 + 120*t),
			uint8(10 + 70*t),
			uint8(30 + 170*t),
			255,
		})
	}
	// 32 lavandas claros.
	for i := 0; i < 32; i++ {
		t := float64(i) / 31
		p = append(p, color.RGBA{
			uint8(180 + 75*t),
			uint8(175 + 80*t),
			255,
			255,
		})
	}
	return p
}
