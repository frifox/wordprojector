//go:build ignore

// Makes the icon of the display (projection) windows: the app icon with a play badge in the bottom right corner.
//
//	go run build/make-display-icon.go internal/shell/icon.png internal/shell/icon-display.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// inTriangle reports whether (x, y) is inside the triangle a, b, c.
func inTriangle(x, y float64, a, b, c [2]float64) bool {
	s := func(p1, p2 [2]float64) float64 { return (x-p2[0])*(p1[1]-p2[1]) - (p1[0]-p2[0])*(y-p2[1]) }
	d1, d2, d3 := s(a, b), s(b, c), s(c, a)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

func main() {
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	src, err := png.Decode(in)
	in.Close()
	if err != nil {
		panic(err)
	}
	b := src.Bounds()
	size := float64(b.Dx())
	out := image.NewNRGBA(b)
	cx, cy := 0.76*size, 0.76*size
	rOuter, rIn := 0.23*size, 0.20*size
	tri := [3][2]float64{{cx - 0.075*size, cy - 0.105*size}, {cx - 0.075*size, cy + 0.105*size}, {cx + 0.115*size, cy}}
	const ss = 4 // samples per pixel side, for smooth edges
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			base := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			var ring, dark, white float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x-b.Min.X) + (float64(sx)+0.5)/ss
					py := float64(y-b.Min.Y) + (float64(sy)+0.5)/ss
					d := math.Hypot(px-cx, py-cy)
					switch {
					case inTriangle(px, py, tri[0], tri[1], tri[2]):
						white++
					case d <= rIn:
						dark++
					case d <= rOuter:
						ring++
					}
				}
			}
			n := float64(ss * ss)
			// the badge over the icon: ring and triangle white, disc dark blue-grey
			c := [3]float64{float64(base.R), float64(base.G), float64(base.B)}
			a := float64(base.A)
			mix := func(col [3]float64, cov float64) {
				if cov == 0 {
					return
				}
				for i := range c {
					c[i] = c[i]*(1-cov) + col[i]*cov
				}
				a = a*(1-cov) + 255*cov
			}
			mix([3]float64{255, 255, 255}, ring/n)
			mix([3]float64{17, 24, 38}, dark/n)
			mix([3]float64{255, 255, 255}, white/n)
			out.SetNRGBA(x, y, color.NRGBA{uint8(c[0] + 0.5), uint8(c[1] + 0.5), uint8(c[2] + 0.5), uint8(a + 0.5)})
		}
	}
	f, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	if err := png.Encode(f, out); err != nil {
		panic(err)
	}
	f.Close()
}
