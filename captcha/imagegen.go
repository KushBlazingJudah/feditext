package captcha

// This really sucks but it works I guess.
// Seriously, turn back now.
//
// Inspiration from https://github.com/dchest/captcha

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"math/rand"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/math/fixed"
)

const (
	CaptchaLen   = 5
	CaptchaIDLen = 16

	shapes    = 20
	maxRadius = 5
)

type img struct {
	width, height int
	*image.Paletted
}

func randIntn(max int) int {
	return rand.Intn(max)

	/* crypto/rand implementation
	   i'd rather use this but it's slow

	i, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		log.Printf("error while reading a random number: %v", err)
		return 4 // chosen by dice roll
	}

	return int(i.Int64()) */
}

func (m *img) drawLine(x, y, ex, ey int, color uint8) {
	if x > ex {
		x, ex = ex, x
	}

	var step float64 = float64(ey-y) / float64(ex-x)
	for i := 0; i < ex-x; i++ {
		y := int(math.Floor(float64(y) + (float64(i) * step)))
		if !(y > m.height || y < 0 || x < 0 || x > m.width) {
			m.SetColorIndex(i+x, y, color)
		}
	}
}

func (m *img) drawLines(amount int, mul int) {
	for i := 0; i < amount; i++ {
		color := uint8(randIntn(shapes-1) + 1)

		for j := 0; j < mul; j++ {
			length := randIntn(100)
			x, y, ey := randIntn(m.width), randIntn(m.height), randIntn(m.height)

			m.drawLine(x, y, x+length, ey, color)
		}
	}
}

func (m *img) drawText(text string) {
	f, err := truetype.Parse(gomonobold.TTF)
	if err != nil {
		panic(err)
	}

	d := &font.Drawer{
		Dst: m,
		Src: image.Black,
		Face: truetype.NewFace(f, &truetype.Options{
			Size:    32,
			DPI:     72,
			Hinting: font.HintingNone,
		}),
	}

	d.Dot = fixed.Point26_6{
		X: (fixed.I(m.width) - d.MeasureString(text)*2 + fixed.I(randIntn(30)-15)) / 2,
		Y: fixed.I((m.height / 2) + int(math.Max(0, float64(randIntn(30)-15)))),
	}

	for i, c := range text {
		bound, adv := d.BoundString(string(c))
		radius := fixed.I(randIntn(maxRadius))

		if bound.Max.X > fixed.I(m.width) || bound.Min.X < 0 {
			d.Dot.X = 0
		}

		if bound.Max.Y > fixed.I(m.height) || bound.Min.Y < 0 {
			d.Dot.Y = fixed.I((m.height / 2) + (randIntn(30) - 15))
		}

		for y := d.Dot.Y - radius; y < d.Dot.Y+radius; y++ {
			for x := d.Dot.X - radius; x < d.Dot.X+radius; x++ {
				color := uint8(randIntn(shapes))
				m.SetColorIndex(x.Ceil(), y.Ceil(), color)
			}
		}

		// I'm sorry
		// Half of this is caused by math not having generics
		boxH := float64(bound.Max.Y.Ceil() - bound.Min.Y.Ceil())
		offs := math.Sin(float64(i+randIntn(3)-1) * 10)
		d.Dot.Y = fixed.I(int(math.Min(math.Max(float64(d.Dot.Y.Ceil())+offs, boxH+10), float64(m.height)-(boxH-10))))

		d.DrawString(string(c))

		for y := d.Dot.Y - radius; y < d.Dot.Y+radius; y++ {
			for x := d.Dot.X - radius; x < d.Dot.X+radius; x++ {
				color := uint8(randIntn(shapes))
				m.SetColorIndex(x.Ceil(), y.Ceil(), color)
			}
		}

		d.Dot.X += adv
	}
}

func (m *img) distort(amplude, period float64) {
	old := m.Paletted
	n := image.NewPaletted(old.Rect, old.Palette)

	dx := 2.0 * math.Pi / period
	for x := 0; x < m.width; x++ {
		for y := 0; y < m.height; y++ {
			xo := amplude * math.Sin(float64(y)*dx)
			yo := amplude * math.Cos(float64(x)*dx)
			n.SetColorIndex(x, y, old.ColorIndexAt(x+int(xo), y+int(yo)))
		}
	}
	m.Paletted = n
}

func randomPalette() color.Palette {
	p := make([]color.Color, shapes+1)

	for i := 0; i < len(p); i++ {
		p[i] = color.RGBA{
			uint8(randIntn(255)),
			uint8(randIntn(255)),
			uint8(randIntn(255)),
			255,
		}
	}

	return p
}

func newImage(width, height int, text string) []byte {
	i := img{width: width, height: height, Paletted: image.NewPaletted(image.Rect(0, 0, width, height), randomPalette())}

	// Set background to a random color that is not 0
	draw.Draw(i, i.Rect, image.White, image.Point{}, draw.Over)

	// Play with this to make captcha harder/easier
	i.drawLines(shapes, 3)
	i.distort(3, 2)
	i.drawText(text)
	i.distort(1, 0.75)
	i.drawLines(shapes/2, 6)

	buf := &bytes.Buffer{}

	// Ignoring error here intentionally; it *shouldn't* fail
	jpeg.Encode(buf, i.Paletted, &jpeg.Options{Quality: 50})

	return buf.Bytes()
}

func captchaText() string {
	hex := []byte("0123456789ABCDEF")
	for i := 0; i < len(hex); i++ {
		src := randIntn(len(hex))
		dest := randIntn(len(hex))
		hex[src], hex[dest] = hex[dest], hex[src]
	}

	return string(hex[:CaptchaLen])
}

func captchaID() string {
	hex := []byte("0123456789ABCDEF")
	out := make([]byte, CaptchaIDLen)

	for i := 0; i < len(out); i++ {
		out[i] = hex[randIntn(len(hex))]
	}

	return string(out[:])
}
