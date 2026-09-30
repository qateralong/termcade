package multi

import (
	"math/rand/v2"
	"sync"
	"time"

	"termcade/internal/games"
	"termcade/internal/ui/theme"
)

// Phase is where a room is in its life.
type Phase int

const (
	Waiting Phase = iota
	Countdown
	Playing
	Finished
)

type seat struct {
	player games.Player
	human  bool
	left   bool   // the human left; a bot plays on
	notify func() // wakes the player's session; nil for bots
	name   string
	color  string
}

// Room is one match and its players.
type Room struct {
	g  *Game
	id int

	mu        sync.RWMutex
	phase     Phase
	seats     []*seat // nil = empty
	deadline  time.Time
	startedAt time.Time
	world     World
	results   []Standing
	closed    bool
	rng       *rand.Rand
}

func newRoom(g *Game, id int) *Room {
	return &Room{
		g:        g,
		id:       id,
		seats:    make([]*seat, g.cfg.Seats),
		deadline: time.Now().Add(g.cfg.Wait),
		rng:      rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64())),
	}
}

// take gives p a seat if the room is still waiting and has space.
func (r *Room) take(p games.Player, notify func()) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != Waiting || r.closed {
		return 0, false
	}
	for i, s := range r.seats {
		if s == nil {
			r.seats[i] = &seat{player: p, human: true, notify: notify, name: p.Name}
			r.assignColors()
			r.wakeLocked()
			return i, true
		}
	}
	return 0, false
}

// leave releases a player's seat. In a waiting room the seat frees up; in a
// running match a bot takes over.
func (r *Room) leave(i int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.seats[i]
	if s == nil || !s.human || s.left {
		return
	}
	if r.phase == Waiting {
		r.seats[i] = nil
		r.assignColors()
	} else {
		s.left = true
		s.notify = nil
		if r.world != nil && r.phase != Finished {
			r.world.SetBot(i)
		}
	}
	r.wakeLocked()
}

// assignColors gives every seated player a distinct color, preferring their
// own choice.
func (r *Room) assignColors() {
	used := map[string]bool{}
	for _, s := range r.seats {
		if s != nil {
			s.color = ""
		}
	}
	for _, s := range r.seats {
		if s == nil || !s.human {
			continue
		}
		if c := theme.PlayerHex(s.player.Color); !used[c] {
			s.color, used[c] = c, true
		}
	}
	for _, s := range r.seats {
		if s == nil || s.color != "" {
			continue
		}
		for _, c := range seatPalette {
			if !used[c] {
				s.color, used[c] = c, true
				break
			}
		}
	}
}

func (r *Room) humansLocked() int {
	n := 0
	for _, s := range r.seats {
		if s != nil && s.human && !s.left {
			n++
		}
	}
	return n
}

// wakeLocked asks every connected player's session to redraw.
func (r *Room) wakeLocked() {
	for _, s := range r.seats {
		if s != nil && s.notify != nil {
			s.notify()
		}
	}
}

// run is the room's clock.
func (r *Room) run() {
	cfg := r.g.cfg
	ticker := time.NewTicker(cfg.Tick)
	defer ticker.Stop()
	last := time.Now()
	for now := range ticker.C {
		dt := now.Sub(last)
		last = now
		if !r.tick(now, dt) {
			r.g.remove(r)
			return
		}
	}
}

// tick advances the room; it returns false once the room should close.
func (r *Room) tick(now time.Time, dt time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.humansLocked() == 0 {
		r.closed = true
		return false
	}
	switch r.phase {
	case Waiting:
		full := true
		for _, s := range r.seats {
			if s == nil {
				full = false
			}
		}
		if full || !now.Before(r.deadline) {
			r.start(now)
		}
	case Countdown:
		if now.Sub(r.startedAt) >= r.g.cfg.Countdown {
			r.phase = Playing
		}
	case Playing:
		r.world.Step(min(dt, 200*time.Millisecond))
		if r.world.Over() {
			r.phase = Finished
			r.results = r.world.Standings()
		}
	case Finished:
		// Keep the room alive while people look at the results.
	}
	r.wakeLocked()
	return true
}

// start fills empty seats with bots and creates the world.
func (r *Room) start(now time.Time) {
	names := append([]string(nil), botNames...)
	r.rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	for i, s := range r.seats {
		if s == nil {
			r.seats[i] = &seat{name: names[i%len(names)]}
		}
	}
	r.assignColors()
	infos := make([]SeatInfo, len(r.seats))
	for i, s := range r.seats {
		infos[i] = SeatInfo{Name: s.name, Color: s.color, Bot: !s.human || s.left}
	}
	r.world = r.g.cfg.NewWorld(infos, r.rng)
	r.phase = Countdown
	r.startedAt = now
}
