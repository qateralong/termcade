package alien

import (
	"math"
	"time"

	"termcade/internal/games/multi"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

var alienSprite = []string{
	"...####...",
	".########.",
	"##EE##EE##",
	"##########",
	".##.##.##.",
	"#..#..#..#",
}

var alienSprite2 = []string{
	"...####...",
	".########.",
	"##EE##EE##",
	"##########",
	".##.##.##.",
	".#..#..#..",
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	c := canvas.New(W, H)
	for _, s := range w.stars {
		c.Set(s[0], s[1], "#3A3452")
	}
	// The orbit.
	for a := 0.0; a < 2*math.Pi; a += 0.09 {
		x, y := pos(a, 1)
		c.Set(int(math.Round(x)), int(math.Round(y)), "#4A3F6B")
	}

	blink := (w.clock/(130*time.Millisecond))%2 == 0
	attacking := false
	for _, h := range w.hazards {
		switch {
		case h.kind == orb:
			w.drawOrb(c, h)
		case h.live(w.clock):
			attacking = true
			w.drawBeam(c, h, h.angle(w.clock), true)
		case w.clock >= h.start && blink:
			w.drawBeam(c, h, h.from, false)
			if h.kind == sweep {
				w.drawBeam(c, h, h.to, false)
			}
		}
	}

	// The alien.
	sprite := alienSprite
	if (w.clock/(400*time.Millisecond))%2 == 1 {
		sprite = alienSprite2
	}
	eye := "#FFFFFF"
	if attacking {
		eye = "#FF4F5E"
	}
	ox, oy := int(cx)-5, int(cy)-3
	for y, row := range sprite {
		for x, ch := range row {
			switch ch {
			case '#':
				c.Set(ox+x, oy+y, "#5BE37D")
			case 'E':
				c.Set(ox+x, oy+y, eye)
			}
		}
	}

	// Saucers: yours on top.
	order := make([]int, 0, len(w.saucers))
	for i := range w.saucers {
		if i != seat {
			order = append(order, i)
		}
	}
	order = append(order, seat)
	for _, i := range order {
		s := w.saucers[i]
		if !s.alive {
			continue
		}
		x, y := pos(s.angle, 1)
		px, py := int(math.Round(x))-2, int(math.Round(y))-1
		dome := theme.Mix(s.info.Color, "#FFFFFF", 0.5)
		if i == seat {
			dome = "#FFFFFF"
		}
		c.Set(px+1, py, dome)
		c.Set(px+2, py, dome)
		for k := 0; k < 4; k++ {
			c.Set(px+k, py+1, s.info.Color)
		}
	}

	for _, b := range w.booms {
		age := w.clock - b.at
		if age > 700*time.Millisecond {
			continue
		}
		r := 1 + int(age/(140*time.Millisecond))
		col := theme.Mix("#FFE066", theme.Coral, float64(age)/float64(700*time.Millisecond))
		for a := 0.0; a < 2*math.Pi; a += 0.6 {
			c.Set(int(b.x+float64(r)*math.Cos(a)), int(b.y+float64(r)*math.Sin(a)), col)
		}
	}

	if me := w.saucers[seat]; !me.alive {
		c.TextCenter(1, " shot down, watching ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.saucers))
	for _, st := range w.Standings() {
		s := w.saucers[st.Seat]
		v := "✓"
		if !s.alive {
			v = "✗"
		}
		rows = append(rows, multi.SidebarRow{Name: s.info.Name, Color: s.info.Color, Value: v, You: st.Seat == seat, Out: !s.alive})
	}
	return multi.Arena(th, c, "ALIVE", itoa(w.alive())+" of "+itoa(len(w.saucers)), rows, "* is you", "space: reverse", "red = incoming")
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// drawBeam draws a ray from the alien out past the orbit. Warnings are
// dashed and red; live beams are solid and bright. Sectors fill a wedge.
func (w *World) drawBeam(c *canvas.Canvas, h *hazard, a float64, live bool) {
	col := "#FF4F5E"
	if live {
		col = "#FFF3A0"
	}
	half := 0.0
	if h.kind == sector {
		half = h.half
	}
	for da := -half; da <= half+1e-9; da += 0.07 {
		for k := 0.3; k <= 1.2; k += 0.03 {
			if !live && int(k*30)%3 == 0 {
				continue // dashed warning
			}
			if h.kind == sector && !live && math.Abs(da) < half-0.05 && int(k*30)%2 == 0 {
				continue
			}
			x, y := pos(a+da, k)
			c.Set(int(math.Round(x)), int(math.Round(y)), col)
		}
	}
}

// drawOrb draws an energy orb flying from the alien to the orbit.
func (w *World) drawOrb(c *canvas.Canvas, h *hazard) {
	if w.clock < h.start || w.clock > h.end {
		return
	}
	travel := float64(h.fire + 60*time.Millisecond - h.start)
	k := 0.3 + 0.7*float64(w.clock-h.start)/travel
	x, y := pos(h.from, k)
	px, py := int(math.Round(x)), int(math.Round(y))
	c.Set(px, py, "#FF6BD6")
	c.Set(px+1, py, "#FF6BD6")
	c.Set(px, py+1, "#C23FA0")
	c.Set(px+1, py+1, "#C23FA0")
}
