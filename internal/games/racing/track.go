package racing

import "math"

type vec struct{ x, y float64 }

func (a vec) add(b vec) vec            { return vec{a.x + b.x, a.y + b.y} }
func (a vec) sub(b vec) vec            { return vec{a.x - b.x, a.y - b.y} }
func (a vec) mul(k float64) vec        { return vec{a.x * k, a.y * k} }
func (a vec) dot(b vec) float64        { return a.x*b.x + a.y*b.y }
func (a vec) len() float64             { return math.Hypot(a.x, a.y) }
func (a vec) dist(b vec) float64       { return a.sub(b).len() }
func lerp(a, b vec, t float64) vec     { return a.add(b.sub(a).mul(t)) }
func angleOf(v vec) float64            { return math.Atan2(v.y, v.x) }
func unit(angle float64) vec           { return vec{math.Cos(angle), math.Sin(angle)} }
func wrapAngle(a float64) float64      { return math.Remainder(a, 2*math.Pi) }
func clampf(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// trackSpec is a closed loop through waypoints, driven in waypoint order.
type trackSpec struct {
	name  string
	width float64
	pts   []vec
}

var tracks = []trackSpec{
	{name: "Oval", width: 7, pts: []vec{
		{14, 6}, {46, 6}, {52, 9}, {54, 18}, {52, 27}, {46, 30}, {14, 30}, {8, 27}, {6, 18}, {8, 9},
	}},
	{name: "Serpent", width: 6, pts: []vec{
		{10, 5}, {50, 5}, {55, 9}, {51, 14}, {22, 14}, {17, 18}, {22, 22}, {50, 22}, {55, 26}, {50, 31}, {10, 31}, {5, 26}, {5, 10},
	}},
	{name: "Kidney", width: 6, pts: []vec{
		{12, 5}, {27, 5}, {31, 11}, {36, 6}, {50, 6}, {55, 14}, {54, 25}, {47, 31}, {32, 31}, {27, 24}, {22, 31}, {10, 30}, {5, 22}, {6, 11},
	}},
}

// track is a trackSpec with precomputed geometry.
type track struct {
	spec   trackSpec
	cum    []float64 // distance along the track at each waypoint
	length float64
	road   [H][W]byte // 0 grass, 1 road, 2 curb
}

func buildTrack(spec trackSpec) *track {
	t := &track{spec: spec}
	n := len(spec.pts)
	t.cum = make([]float64, n+1)
	for i := 0; i < n; i++ {
		t.cum[i+1] = t.cum[i] + spec.pts[i].dist(spec.pts[(i+1)%n])
	}
	t.length = t.cum[n]
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			p := vec{float64(x) + 0.5, float64(y) + 0.5}
			_, d := t.project(p)
			switch {
			case d <= spec.width/2:
				t.road[y][x] = 1
			case d <= spec.width/2+1:
				t.road[y][x] = 2
			}
		}
	}
	return t
}

// project returns the distance along the track of the point closest to p,
// and how far p is from the centerline.
func (t *track) project(p vec) (along, off float64) {
	n := len(t.spec.pts)
	best := math.Inf(1)
	for i := 0; i < n; i++ {
		a, b := t.spec.pts[i], t.spec.pts[(i+1)%n]
		ab := b.sub(a)
		u := clampf(p.sub(a).dot(ab)/ab.dot(ab), 0, 1)
		q := lerp(a, b, u)
		if d := p.dist(q); d < best {
			best = d
			along = t.cum[i] + u*ab.len()
		}
	}
	return along, best
}

// pointAt returns the centerline point at a distance along the track.
func (t *track) pointAt(s float64) vec {
	s = math.Mod(s, t.length)
	if s < 0 {
		s += t.length
	}
	n := len(t.spec.pts)
	for i := 0; i < n; i++ {
		if s <= t.cum[i+1] {
			a, b := t.spec.pts[i], t.spec.pts[(i+1)%n]
			return lerp(a, b, (s-t.cum[i])/(t.cum[i+1]-t.cum[i]))
		}
	}
	return t.spec.pts[0]
}

// surface reports what's under a position: 1 road, 2 curb, 0 grass.
func (t *track) surface(p vec) byte {
	x, y := int(p.x), int(p.y)
	if x < 0 || y < 0 || x >= W || y >= H {
		return 0
	}
	return t.road[y][x]
}

// direction is the heading of the track at a distance along it.
func (t *track) direction(s float64) float64 {
	return angleOf(t.pointAt(s + 1).sub(t.pointAt(s - 1)))
}

// wrap maps a difference in track distance into (-length/2, length/2].
func (t *track) wrap(d float64) float64 {
	d = math.Mod(d, t.length)
	if d > t.length/2 {
		d -= t.length
	} else if d <= -t.length/2 {
		d += t.length
	}
	return d
}
