package bossrush

import (
	"math"
	"testing"
	"time"
)

// dodger plays the heart: it tries each move and picks the one that stays
// clear of bullets and beams for the next moment.
func dodger(e *Engine) {
	type option struct {
		key    string
		dx, dy float64
	}
	opts := []option{{"", 0, 0}, {"left", -2, 0}, {"right", 2, 0}, {"up", 0, -1}, {"down", 0, 1}}
	if e.blue {
		opts = []option{{"", 0, 0}, {"left", -2, 0}, {"right", 2, 0}, {"up", 0, -2}}
	}
	bestKey, bestRisk := "", math.MaxInt
	for _, o := range opts {
		x := math.Max(0, math.Min(BoxW-1, e.hx+o.dx))
		y := math.Max(0, math.Min(BoxH-1, e.hy+o.dy))
		risk := 0
		for step := 0.0; step <= 0.4; step += 0.05 {
			for _, b := range e.bullets {
				fb := *b
				fb.x += b.vx * step
				fb.y += b.vy * step
				if fb.hits(int(math.Round(x)), int(math.Round(y))) {
					risk += 10
				}
			}
			at := e.att.elapsed + time.Duration(step*float64(time.Second))
			for _, bm := range e.beams {
				if at >= bm.fireAt-100*time.Millisecond && at <= bm.endAt &&
					((bm.horiz && bm.pos == int(math.Round(y))) || (!bm.horiz && bm.pos == int(math.Round(x)))) {
					risk += 10
				}
			}
		}
		if risk < bestRisk {
			bestKey, bestRisk = o.key, risk
		}
	}
	if bestKey != "" {
		e.Key(bestKey)
	}
}

// TestAttacksAreDodgeable checks that a simple dodging autopilot gets
// through each boss's attacks without losing most of its health; a person
// who sees the warnings can do better.
func TestAttacksAreDodgeable(t *testing.T) {
	for bi, b := range bosses {
		for _, k := range b.attacks {
			hits, runs := 0, 6
			for r := 0; r < runs; r++ {
				e := newTest(uint64(bi*100 + int(k)*10 + r))
				e.meet(bi)
				e.advance()
				e.startAttack()
				e.att.kind = k
				e.blue = k == boneHop
				if e.blue {
					e.hy = BoxH - 1
				}
				e.hp = 1 << 20
				for e.phase == dodge {
					if int(e.clock/(25*time.Millisecond))%2 == 0 {
						dodger(e)
					}
					before := e.hp
					e.Update(25 * time.Millisecond)
					if e.hp < before {
						hits++
					}
				}
			}
			avg := float64(hits) / float64(runs)
			t.Logf("%-12s attack %d: %.1f hits per turn", b.name, k, avg)
			if avg > 3.5 {
				t.Errorf("%s attack %d looks too hard: %.1f hits per turn", b.name, k, avg)
			}
		}
	}
}
