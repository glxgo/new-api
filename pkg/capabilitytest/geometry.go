package capabilitytest

import (
	"image"
	"math"
	"strconv"
)

// GradeGeometry combines the constrained scene graph with raster evidence.
// Missing rasterization never becomes a passing geometry score.
func GradeGeometry(shapes []Shape, img image.Image, label string) []bool {
	out := make([]bool, 5)
	if img == nil || img.Bounds().Dx() != 600 || img.Bounds().Dy() != 400 {
		return out
	}
	type circle struct{ x, y, r float64 }
	type box struct{ x, y, w, h float64 }
	cs := []circle{}
	rs := []box{}
	colors, position, visible, text := true, true, true, false
	background := false
	num := func(a map[string]string, k string) float64 {
		v, err := strconv.ParseFloat(a[k], 64)
		if err != nil {
			return math.NaN()
		}
		return v
	}
	for i, s := range shapes {
		a := s.Attr
		switch s.Tag {
		case "circle":
			c := circle{num(a, "cx"), num(a, "cy"), num(a, "r")}
			cs = append(cs, c)
			colors = colors && a["fill"] == "#ef4444"
			position = position && c.x-c.r >= 0 && c.x+c.r <= 300 && c.y-c.r >= 0 && c.y+c.r < 330
			visible = visible && c.r >= 15 && c.r <= 150
			if !pixelNear(img, int(c.x), int(c.y), 239, 68, 68) {
				visible = false
			}
		case "rect":
			r := box{num(a, "x"), num(a, "y"), num(a, "width"), num(a, "height")}
			if i == 1 && r.x == 0 && r.y == 0 && r.w == 600 && r.h == 400 && (a["fill"] == "white" || a["fill"] == "#ffffff") {
				background = true
				continue
			}
			rs = append(rs, r)
			colors = colors && a["fill"] == "#3b82f6"
			position = position && r.x >= 300 && r.x+r.w <= 600 && r.y >= 0 && r.y+r.h < 330
			visible = visible && r.w >= 30 && r.w == r.h && a["rx"] == "" && a["ry"] == ""
			if !pixelNear(img, int(r.x+r.w/2), int(r.y+r.h/2), 59, 130, 246) {
				visible = false
			}
		case "text":
			y, x, size := num(a, "y"), num(a, "x"), num(a, "font-size")
			text = s.Text == label && y >= 360 && y <= 390 && x >= 0 && x <= 500 && size == 20 && (a["fill"] == "black" || a["fill"] == "#000000")
		}
	}
	for i, c := range cs {
		for _, other := range cs[i+1:] {
			if math.Hypot(c.x-other.x, c.y-other.y) <= c.r+other.r {
				visible = false
			}
		}
	}
	for i, r := range rs {
		for _, other := range rs[i+1:] {
			if r.x < other.x+other.w && r.x+r.w > other.x && r.y < other.y+other.h && r.y+r.h > other.y {
				visible = false
			}
		}
	}
	blackPixels := 0
	for y := 340; y < 400; y++ {
		for x := 0; x < 600; x++ {
			if pixelNear(img, x, y, 0, 0, 0) {
				blackPixels++
			}
		}
	}
	texts := 0
	for _, s := range shapes {
		if s.Tag == "text" {
			texts++
		}
	}
	out[0] = len(cs) == 3 && len(rs) == 2
	out[1] = colors && background && len(cs) > 0 && len(rs) > 0
	out[2] = position && len(cs) > 0 && len(rs) > 0
	out[3] = visible && position && background
	out[4] = text && texts == 1 && blackPixels > 20
	return out
}
func pixelNear(img image.Image, x, y int, r, g, b uint32) bool {
	if !(image.Point{X: x, Y: y}.In(img.Bounds())) {
		return false
	}
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return ca >= 65000 && math.Abs(float64(cr>>8)-float64(r)) < 12 && math.Abs(float64(cg>>8)-float64(g)) < 12 && math.Abs(float64(cb>>8)-float64(b)) < 12
}
