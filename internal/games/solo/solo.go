// Package solo is a small framework for single-player games. A game supplies
// an Engine with its rules and drawing; solo provides everything around it:
// the title card with controls and leaderboard, the game loop, pausing, the
// game-over screen, score keeping and the frame with header and key hints.
package solo

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"termcade/internal/games"
)

// State is the outcome of a game so far.
type State int

const (
	Playing State = iota
	Won
	Lost
)

// Stat is a label and value shown in the header, e.g. LEVEL 3.
type Stat struct {
	Label string
	Value string
}

// Engine is one game in progress.
type Engine interface {
	// Update advances the simulation by dt.
	Update(dt time.Duration)
	// Key handles a gameplay key, as reported by tea.KeyMsg.String().
	Key(key string)
	// View draws the play area. Its size must not depend on the game state,
	// so the frame around it stays put.
	View() string
	Score() int
	State() State
	// Stats are extra header values such as level or lives.
	Stats() []Stat
}

// MouseEngine is implemented by engines that accept mouse input. x and y are
// relative to the top-left corner of View.
type MouseEngine interface {
	Mouse(x, y int, button tea.MouseButton)
}

// Variant is a selectable flavor of a game, such as a difficulty. Each
// variant has its own leaderboard.
type Variant struct {
	Key  string
	Name string
	Desc string
}

// Config describes a solo game.
type Config struct {
	Info     games.Info
	Variants []Variant
	// New starts a game of the given variant ("" if there are none).
	New func(s games.Session, variant string, rng *rand.Rand) Engine
	// Tick is the simulation step; defaults to 50ms.
	Tick time.Duration
	// LowerIsBetter ranks scores ascending, for games scored by time.
	LowerIsBetter bool
	// OnlyWins records scores of won games only.
	OnlyWins bool
	// ScoreLabel names the score in the header; defaults to "SCORE".
	ScoreLabel string
	// FormatScore renders a score; defaults to digits with separators.
	FormatScore func(int) string
	// Mouse enables mouse reporting while the game is open.
	Mouse bool
}

// Game adapts a Config to games.Game.
type Game struct{ cfg Config }

// New returns a solo game.
func New(cfg Config) *Game {
	cfg.Info.Mode = games.Solo
	if cfg.Tick == 0 {
		cfg.Tick = 50 * time.Millisecond
	}
	if cfg.ScoreLabel == "" {
		cfg.ScoreLabel = "SCORE"
	}
	if cfg.FormatScore == nil {
		cfg.FormatScore = Thousands
	}
	return &Game{cfg: cfg}
}

// Info implements games.Game.
func (g *Game) Info() games.Info { return g.cfg.Info }

// Available implements games.Game.
func (g *Game) Available() bool { return true }

// Join implements games.Game.
func (g *Game) Join(s games.Session) tea.Model {
	m := &model{
		cfg: g.cfg,
		s:   s,
		w:   s.Width,
		h:   s.Height,
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64())),
	}
	m.loadBoard()
	return m
}

// Thousands formats n with comma separators: 12345 -> "12,345".
func Thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// Duration formats milliseconds as "1:05.3", or "42.0" under a minute.
func Duration(ms int) string {
	d := time.Duration(ms) * time.Millisecond
	tenths := int(d/(100*time.Millisecond)) % 10
	secs := int(d/time.Second) % 60
	mins := int(d / time.Minute)
	if mins > 0 {
		return strconv.Itoa(mins) + ":" + pad2(secs) + "." + strconv.Itoa(tenths)
	}
	return strconv.Itoa(secs) + "." + strconv.Itoa(tenths)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
