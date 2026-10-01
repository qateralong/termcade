package bossrush

import (
	"math"
	"time"
)

// The bullet box, in character cells.
const (
	BoxW = 36
	BoxH = 10
)

type attackKind int

const (
	boneWall attackKind = iota // vertical bones sweep across with a gap
	boneHop                    // gravity mode: jump over bones on the floor
	fireRain                   // fireballs fall from the top
	spiral                     // bullets spin out from the center
	aimed                      // shots from the edges, aimed at you
	laser                      // warned rows and columns light up
	ring                       // a ring closes in, with one gap
)

// bullet is a moving hazard occupying a rectangle of cells.
type bullet struct {
	x, y   float64
	vx, vy float64
	w, h   int
	glyph  string
	color  string
}

func (b *bullet) hits(x, y int) bool {
	bx, by := int(math.Floor(b.x)), int(math.Floor(b.y))
	return x >= bx && x < bx+b.w && y >= by && y < by+b.h
}

func (b *bullet) gone() bool {
	return b.x+float64(b.w) < -2 || b.x > BoxW+2 || b.y+float64(b.h) < -2 || b.y > BoxH+2
}

// beam is a laser across a whole row or column.
type beam struct {
	horiz  bool
	pos    int
	fireAt time.Duration
	endAt  time.Duration
}

// attack is the boss's turn in progress.
type attack struct {
	kind     attackKind
	elapsed  time.Duration
	duration time.Duration
	next     time.Duration // when to spawn the next wave
	n        int           // waves spawned so far
}

const (
	boneColor = "#F2F2F2"
	fireColor = "#FF9F43"
	shotColor = "#FFE066"
	ringColor = "#9D7BFF"
)

// spawn adds the next wave of an attack. sp scales speed and density.
func (e *Engine) spawn(a *attack, sp float64) {
	a.n++
	switch a.kind {
	case boneWall:
		gap := 3
		top := 1 + e.rng.IntN(BoxH-gap-1)
		for y := 0; y < BoxH; y++ {
			if y >= top && y < top+gap {
				continue
			}
			e.bullets = append(e.bullets, &bullet{x: BoxW, y: float64(y), vx: -11 * sp, w: 1, h: 1, glyph: "┃", color: boneColor})
		}
		a.next += time.Duration(float64(1000*time.Millisecond) / sp)
	case boneHop:
		h := 1 + e.rng.IntN(2)
		e.bullets = append(e.bullets, &bullet{x: BoxW, y: float64(BoxH - h), vx: -13 * sp, w: 1, h: h, glyph: "┃", color: boneColor})
		if e.rng.IntN(3) == 0 { // a ceiling bone to duck under
			e.bullets = append(e.bullets, &bullet{x: BoxW + 6, y: 0, vx: -13 * sp, w: 1, h: BoxH - 4, glyph: "┃", color: boneColor})
		}
		a.next += time.Duration(float64(time.Duration(900+e.rng.IntN(500))*time.Millisecond) / sp)
	case fireRain:
		for i := 0; i < 2; i++ {
			e.bullets = append(e.bullets, &bullet{
				x: float64(e.rng.IntN(BoxW)), y: -1,
				vx: (e.rng.Float64() - 0.5) * 3, vy: (5 + e.rng.Float64()*5) * sp,
				w: 1, h: 1, glyph: "●", color: fireColor,
			})
		}
		a.next += time.Duration(float64(280*time.Millisecond) / sp)
	case spiral:
		base := float64(a.n) * 0.45
		for k := 0; k < 3; k++ {
			ang := base + float64(k)*2*math.Pi/3
			e.bullets = append(e.bullets, &bullet{
				x: BoxW / 2, y: BoxH / 2,
				vx: math.Cos(ang) * 9 * sp, vy: math.Sin(ang) * 4.5 * sp,
				w: 1, h: 1, glyph: "◆", color: e.boss().color,
			})
		}
		a.next += time.Duration(float64(200*time.Millisecond) / sp)
	case aimed:
		// From a random edge toward the heart.
		var x, y float64
		switch e.rng.IntN(4) {
		case 0:
			x, y = -1, float64(e.rng.IntN(BoxH))
		case 1:
			x, y = BoxW, float64(e.rng.IntN(BoxH))
		case 2:
			x, y = float64(e.rng.IntN(BoxW)), -1
		default:
			x, y = float64(e.rng.IntN(BoxW)), BoxH
		}
		dx, dy := e.hx-x, (e.hy-y)*2 // cells are twice as tall as wide
		d := math.Hypot(dx, dy)
		v := 14 * sp
		e.bullets = append(e.bullets, &bullet{x: x, y: y, vx: dx / d * v, vy: dy / d * v / 2, w: 1, h: 1, glyph: "✦", color: shotColor})
		a.next += time.Duration(float64(380*time.Millisecond) / sp)
	case laser:
		warn := time.Duration(float64(900*time.Millisecond) / sp)
		n := 1 + e.rng.IntN(2)
		for i := 0; i < n; i++ {
			b := &beam{horiz: e.rng.IntN(2) == 0, fireAt: a.elapsed + warn, endAt: a.elapsed + warn + 400*time.Millisecond}
			if b.horiz {
				b.pos = e.rng.IntN(BoxH)
			} else {
				b.pos = e.rng.IntN(BoxW)
			}
			e.beams = append(e.beams, b)
		}
		a.next += time.Duration(float64(1100*time.Millisecond) / sp)
	case ring:
		gapAng := e.rng.Float64() * 2 * math.Pi
		for k := 0; k < 24; k++ {
			ang := float64(k) * 2 * math.Pi / 24
			if math.Abs(math.Remainder(ang-gapAng, 2*math.Pi)) < 0.5 {
				continue
			}
			x := BoxW/2 + math.Cos(ang)*BoxW*0.7
			y := BoxH/2 + math.Sin(ang)*BoxH*0.7
			e.bullets = append(e.bullets, &bullet{
				x: x, y: y, vx: -math.Cos(ang) * 6 * sp, vy: -math.Sin(ang) * 3 * sp,
				w: 1, h: 1, glyph: "●", color: ringColor,
			})
		}
		a.next += time.Duration(float64(2200*time.Millisecond) / sp)
	}
}
