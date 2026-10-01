package chickenrun

import "time"

const (
	botThink    = 130 * time.Millisecond
	botLookHead = 1400 * time.Millisecond
	botMistakes = 12 // percent chance to ignore a good flip
)

// think decides whether a bot chicken should flip: it simulates the next
// moment both ways and picks whichever keeps it alive and moving longer.
func (w *World) think(c *chicken) {
	if w.clock < c.thinkAt || !w.grounded(c) {
		return
	}
	c.thinkAt = w.clock + botThink
	stay := w.simulate(*c, false)
	flip := w.simulate(*c, true)
	if flip > stay+2 && w.rng.IntN(100) >= botMistakes {
		w.flip(c, 0)
	}
}

// simulate plays a copy of c forward and scores the outcome: how far it got,
// with a big penalty for dying.
func (w *World) simulate(c chicken, flipNow bool) float64 {
	if flipNow {
		c.grav = -c.grav
		c.vy = 0
	}
	const step = 50 * time.Millisecond
	cam := w.camX
	startX := c.x
	for t := time.Duration(0); t < botLookHead; t += step {
		if w.physics(&c, step.Seconds(), w.speed, w.clock+t+step) != "" {
			return c.x - startX - 1000
		}
		cam += w.speed * step.Seconds()
		if c.x+size < cam-8 {
			return c.x - startX - 500
		}
		// Once past the danger, a later flip could still help; keep it
		// simple and let the next decision handle that.
	}
	return c.x - startX
}
