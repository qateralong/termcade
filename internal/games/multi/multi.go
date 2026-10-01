// Package multi runs multiplayer rooms for realtime games.
//
// Matchmaking works the same for every game: the first player to join a room
// starts a countdown. The room starts as soon as every seat is taken, when
// every player in it says they're ready, or when the countdown runs out; bots
// fill any empty seats. A
// room that is already playing never takes new players; a new room is made
// instead. A player who leaves mid-game is replaced by a bot.
//
// A game provides a World: the rules, the bots and the drawing. The room runs
// the World on its own goroutine at a fixed tick rate, and every player's
// session draws the World from its own seat's point of view.
package multi

import (
	"math/rand/v2"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"termcade/internal/games"
	"termcade/internal/ui/theme"
)

// SeatInfo describes who sits in a seat when a World is created.
type SeatInfo struct {
	Name  string
	Color string // hex
	Bot   bool
}

// Standing is a line of the final results, best first.
type Standing struct {
	Seat   int
	Detail string // e.g. "42 points" or "1:23.4"
}

// Stat is a label and value shown in the header.
type Stat struct {
	Label string
	Value string
}

// World is one match of a game. The room serializes all calls: Input and
// Step never run concurrently with each other or with View.
type World interface {
	// Input handles a key pressed by the player in seat.
	Input(seat int, key string)
	// Step advances the match by dt, including moving the bots.
	Step(dt time.Duration)
	// SetBot hands a seat over to a bot, e.g. when its player leaves.
	SetBot(seat int)
	// Over reports whether the match has finished.
	Over() bool
	// Standings ranks the seats, best first.
	Standings() []Standing
	// View draws the match as seen from seat. The size must be fixed.
	View(seat int, th *theme.Theme) string
	// Stats are header values for seat, e.g. score or lap.
	Stats(seat int) []Stat
}

// Config describes a multiplayer game.
type Config struct {
	Info  games.Info
	Seats int
	// Wait is how long a room waits for players; defaults to 30s.
	Wait time.Duration
	// Countdown is the 3-2-1 before play; defaults to 3s.
	Countdown time.Duration
	// Tick is the simulation step; defaults to 50ms.
	Tick time.Duration
	// NewWorld starts a match.
	NewWorld func(seats []SeatInfo, rng *rand.Rand) World
}

// Game implements games.Game and owns the rooms.
type Game struct {
	cfg Config

	mu     sync.Mutex
	rooms  []*Room
	nextID int

	manual bool // tests drive rooms by hand instead of with a clock
}

// New returns a multiplayer game.
func New(cfg Config) *Game {
	cfg.Info.Mode = games.Multiplayer
	cfg.Info.Description += " Rooms start when full, when everyone is ready, or after 30 seconds, with bots in the empty seats."
	if cfg.Wait == 0 {
		cfg.Wait = 30 * time.Second
	}
	if cfg.Countdown == 0 {
		cfg.Countdown = 3 * time.Second
	}
	if cfg.Tick == 0 {
		cfg.Tick = 50 * time.Millisecond
	}
	return &Game{cfg: cfg}
}

// Info implements games.Game.
func (g *Game) Info() games.Info { return g.cfg.Info }

// Available implements games.Game.
func (g *Game) Available() bool { return true }

// Join implements games.Game.
func (g *Game) Join(s games.Session) tea.Model {
	return newModel(g, s)
}

// Rooms returns the number of open rooms (for tests and stats).
func (g *Game) Rooms() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.rooms)
}

// seatFor puts a player in the first waiting room with a free seat, or in a
// new room.
func (g *Game) seatFor(p games.Player, notify func()) (*Room, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.rooms {
		if seat, ok := r.take(p, notify); ok {
			return r, seat
		}
	}
	g.nextID++
	r := newRoom(g, g.nextID)
	g.rooms = append(g.rooms, r)
	seat, _ := r.take(p, notify)
	if !g.manual {
		go r.run()
	}
	return r, seat
}

func (g *Game) remove(r *Room) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, x := range g.rooms {
		if x == r {
			g.rooms = append(g.rooms[:i], g.rooms[i+1:]...)
			return
		}
	}
}

// Bot names, handed out in order.
var botNames = []string{"Byte", "Pixel", "Glitch", "Turbo", "Nova", "Blip", "Chip", "Echo", "Zap", "Rex"}

// Seat colors: players keep their own color unless someone in the room
// already has it.
var seatPalette = []string{theme.Pink, theme.Cyan, theme.Lime, theme.Amber, theme.Violet, theme.Coral, theme.Sky, theme.Mint}
