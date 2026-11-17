// Command icongen draws the app icon: two overlapping profile silhouettes on a
// warm rounded square. It writes winres/icon.ico (all Windows icon sizes) and
// PNG previews. Run from the repository root:
//
//	go run ./tools/icongen
//
// then `go run github.com/tc-hib/go-winres@v0.3.3 make` to rebuild the .syso
// resources that embed the icon into the .exe files.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

type rgba struct{ r, g, b, a float64 } // premultiplied, 0..1

func over(dst, src rgba) rgba {
	k := 1 - src.a
	return rgba{src.r + dst.r*k, src.g + dst.g*k, src.b + dst.b*k, src.a + dst.a*k}
}

func solid(r, g, b, a float64) rgba { return rgba{r * a, g * a, b * a, a} }

// Signed distance helpers (negative inside), in unit coordinates.
func circle(x, y, cx, cy, r float64) float64 { return math.Hypot(x-cx, y-cy) - r }

func roundRect(x, y, x0, y0, x1, y1, rad float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-rad, (y1-y0)/2-rad
	dx, dy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	out := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	return out + math.Min(math.Max(dx, dy), 0) - rad
}

// person is a head plus rounded shoulders, cut flat at the bottom.
func person(x, y, cx, top, s float64) float64 {
	head := circle(x, y, cx, top+0.13*s, 0.13*s)
	body := circle(x, y, cx, top+0.50*s, 0.25*s)
	body = math.Max(body, y-(top+0.47*s)) // flat cut
	return math.Min(head, body)
}

func sample(x, y float64) rgba {
	const inset = 0.035
	bgD := roundRect(x, y, inset, inset, 1-inset, 1-inset, 0.23)
	if bgD > 0 {
		return rgba{}
	}
	// Warm vertical gradient.
	t := (y - inset) / (1 - 2*inset)
	top := [3]float64{0xE0, 0x82, 0x55}
	bot := [3]float64{0xA8, 0x45, 0x28}
	mix := func(i int) float64 { return (top[i]*(1-t) + bot[i]*t) / 255 }
	px := solid(mix(0), mix(1), mix(2), 1)

	back := person(x, y, 0.635, 0.20, 1.0)
	front := person(x, y, 0.405, 0.29, 1.12)
	const gap = 0.04

	if back < 0 && front > gap { // back silhouette, separated from the front one
		px = over(px, solid(1, 0.93, 0.88, 0.62))
	}
	if front < 0 {
		px = over(px, solid(1, 1, 1, 1))
	}
	return px
}

func render(size int) *image.NRGBA {
	const ss = 8 // supersampling per axis
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var acc rgba
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					c := sample(x, y)
					acc.r += c.r
					acc.g += c.g
					acc.b += c.b
					acc.a += c.a
				}
			}
			n := float64(ss * ss)
			a := acc.a / n
			if a <= 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(acc.r / n / a * 255)),
				G: uint8(math.Round(acc.g / n / a * 255)),
				B: uint8(math.Round(acc.b / n / a * 255)),
				A: uint8(math.Round(a * 255)),
			})
		}
	}
	return img
}

func pngBytes(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

// dib encodes an image as a 32-bit icon bitmap (BITMAPINFOHEADER, bottom-up
// BGRA, then an all-zero AND mask), which every Windows component can read.
func dib(img *image.NRGBA) []byte {
	s := img.Bounds().Dx()
	maskStride := (s + 31) / 32 * 4
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Size                        uint32
		Width, Height               int32
		Planes, BPP                 uint16
		Compression, ImageSize      uint32
		XPPM, YPPM, Used, Important int32
	}{40, int32(s), int32(2 * s), 1, 32, 0, uint32(s*s*4 + maskStride*s), 0, 0, 0, 0})
	for y := s - 1; y >= 0; y-- {
		for x := 0; x < s; x++ {
			c := img.NRGBAAt(x, y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, maskStride*s))
	return b.Bytes()
}

// writeICO stores sizes below 256 as bitmaps and 256 as PNG, as Windows does.
func writeICO(path string, sizes []int) {
	var imgs [][]byte
	for _, s := range sizes {
		if s >= 256 {
			imgs = append(imgs, pngBytes(render(s)))
		} else {
			imgs = append(imgs, dib(render(s)))
		}
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0
		}
		binary.Write(&b, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(imgs[i])), uint32(offset)})
		offset += len(imgs[i])
	}
	for _, d := range imgs {
		b.Write(d)
	}
	must(os.WriteFile(path, b.Bytes(), 0o644))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	must(os.MkdirAll("winres", 0o755))
	must(os.MkdirAll("docs", 0o755))
	writeICO(filepath.Join("winres", "icon.ico"), []int{16, 20, 24, 32, 40, 48, 64, 128, 256})
	must(os.WriteFile(filepath.Join("winres", "icon.png"), pngBytes(render(256)), 0o644))
	must(os.WriteFile(filepath.Join("docs", "icon.png"), pngBytes(render(512)), 0o644))
	log.Println("wrote winres/icon.ico, winres/icon.png, docs/icon.png")
}
