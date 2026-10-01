package alien

import (
	"time"
)

const (
	botEvery    = 80 * time.Millisecond
	botMistakes = 12 // percent
)

// think reverses a bot saucer when it sees that staying on course will get
// it hit and turning around won't. Bots only notice an attack after a short,
// human-like reaction time.
func (w *World) think(s *saucer) {
	if w.clock < s.nextBot {
		return
	}
	s.nextBot = w.clock + botEvery
	var seen []*hazard
	for _, h := range w.hazards {
		at, ok := s.reactAt[h]
		if !ok {
			at = h.start + time.Duration(260+w.rng.IntN(380))*time.Millisecond
			s.reactAt[h] = at
		}
		if w.clock >= at {
			seen = append(seen, h)
		}
	}
	if len(seen) == 0 {
		return
	}
	stay := w.danger(s.angle, s.dir, seen)
	flip := w.danger(s.angle, -s.dir, seen)
	if flip < stay && w.rng.IntN(100) >= botMistakes {
		s.dir = -s.dir
	}
}

// danger counts how many moments over the next couple of seconds a saucer
// moving in dir would be caught by one of the hazards.
func (w *World) danger(a, dir float64, hs []*hazard) int {
	n := 0
	const step = 40 * time.Millisecond
	for t := time.Duration(0); t < 2200*time.Millisecond; t += step {
		at := w.clock + t
		ang := wrap(a + dir*orbitSpeed*t.Seconds())
		for _, h := range hs {
			if h.hits(ang, at) {
				n++
			}
		}
	}
	return n
}
