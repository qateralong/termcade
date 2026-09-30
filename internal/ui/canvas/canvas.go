// Package canvas draws pixel graphics in a terminal using half blocks: each
// character cell shows two vertically stacked pixels ("▀" with a foreground
// and a background color), so pixels come out roughly square.
package canvas

import (
	"strings"

	"github.com/muesli/termenv"
)

// Canvas is a grid of pixels. The zero color ("") is transparent and shows
// the terminal background.
type Canvas struct {
	W, H int
	pix  []string
	// Text overlays, drawn over whole character cells.
	text map[[2]int]textCell
}

type textCell struct {
	r      rune
	fg, bg string
}

// New returns a blank canvas. H should be even.
func New(w, h int) *Canvas {
	return &Canvas{W: w, H: h, pix: make([]string, w*h)}
}

// Set colors one pixel. Out-of-range pixels are ignored.
func (c *Canvas) Set(x, y int, color string) {
	if x >= 0 && y >= 0 && x < c.W && y < c.H {
		c.pix[y*c.W+x] = color
	}
}

// Get returns a pixel's color.
func (c *Canvas) Get(x, y int) string {
	if x >= 0 && y >= 0 && x < c.W && y < c.H {
		return c.pix[y*c.W+x]
	}
	return ""
}

// Fill colors a rectangle.
func (c *Canvas) Fill(x, y, w, h int, color string) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.Set(xx, yy, color)
		}
	}
}

// Text writes a string over character cells, starting at pixel column x
// and character row row (each row is two pixels tall). An empty bg keeps
// the pixels behind the text visible as the background.
func (c *Canvas) Text(x, row int, s string, fg, bg string) {
	if c.text == nil {
		c.text = make(map[[2]int]textCell)
	}
	for _, r := range s {
		if x >= 0 && x < c.W && row >= 0 && row < (c.H+1)/2 {
			c.text[[2]int{x, row}] = textCell{r, fg, bg}
		}
		x++
	}
}

// TextCenter writes s centered horizontally on a character row.
func (c *Canvas) TextCenter(row int, s string, fg, bg string) {
	c.Text((c.W-len([]rune(s)))/2, row, s, fg, bg)
}

// Render turns the canvas into a string for the given color profile.
func (c *Canvas) Render(p termenv.Profile) string {
	seq := map[string][2]string{} // hex -> {fg, bg} SGR parameters
	colorSeq := func(hex string) [2]string {
		if s, ok := seq[hex]; ok {
			return s
		}
		col := p.Color(hex)
		var s [2]string
		if col != nil {
			s = [2]string{col.Sequence(false), col.Sequence(true)}
		}
		seq[hex] = s
		return s
	}

	var b strings.Builder
	b.Grow(c.W * c.H * 8)
	rows := (c.H + 1) / 2
	for row := 0; row < rows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		last := ""
		for x := 0; x < c.W; x++ {
			var glyph rune
			var fg, bg string
			if t, ok := c.text[[2]int{x, row}]; ok {
				glyph, fg, bg = t.r, t.fg, t.bg
				if bg == "" {
					bg = c.Get(x, row*2+1)
					if bg == "" {
						bg = c.Get(x, row*2)
					}
				}
			} else {
				top, bottom := c.Get(x, row*2), c.Get(x, row*2+1)
				switch {
				case top == "" && bottom == "":
					glyph = ' '
				case top == bottom:
					glyph, fg = '█', top
				case bottom == "":
					glyph, fg = '▀', top
				case top == "":
					glyph, fg = '▄', bottom
				default:
					glyph, fg, bg = '▀', top, bottom
				}
			}

			var sgr string
			if fg != "" || bg != "" {
				parts := []string{}
				if fg != "" {
					if s := colorSeq(fg)[0]; s != "" {
						parts = append(parts, s)
					}
				}
				if bg != "" {
					if s := colorSeq(bg)[1]; s != "" {
						parts = append(parts, s)
					}
				}
				sgr = strings.Join(parts, ";")
			}
			if sgr != last {
				b.WriteString("\x1b[0m")
				if sgr != "" {
					b.WriteString("\x1b[" + sgr + "m")
				}
				last = sgr
			}
			b.WriteRune(glyph)
		}
		if last != "" {
			b.WriteString("\x1b[0m")
		}
	}
	return b.String()
}
