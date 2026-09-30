package racing

import "math"

// drive steers a bot toward a point a little way ahead on the track and
// lifts off before sharp turns.
func (w *World) drive(c *car) {
	along, _ := w.tr.project(c.pos)
	target := w.tr.pointAt(along + 5 + c.speed*0.25)
	want := angleOf(target.sub(c.pos))
	diff := wrapAngle(want - c.angle())
	step := 2 * math.Pi / turnSteps
	switch {
	case diff > step/2:
		c.heading = (c.heading + 1) % turnSteps
	case diff < -step/2:
		c.heading = (c.heading + turnSteps - 1) % turnSteps
	}

	// How sharply does the track bend just ahead?
	a := w.tr.pointAt(along + 4)
	b := w.tr.pointAt(along + 12)
	bend := math.Abs(wrapAngle(angleOf(b.sub(a)) - angleOf(a.sub(c.pos))))

	limit := topSpeed * c.skill
	if bend > 0.6 {
		limit *= 0.6
	} else if bend > 0.3 {
		limit *= 0.8
	}
	if math.Abs(diff) > 0.8 {
		limit = math.Min(limit, 7)
	}
	switch {
	case c.speed < limit:
		c.gas, c.brakeT = w.clock+pedalGrant, 0
	case c.speed > limit+3:
		c.brakeT, c.gas = w.clock+pedalGrant/3, 0
	default:
		c.gas, c.brakeT = 0, 0
	}
}
