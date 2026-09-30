// Package games defines the contract between the arcade platform and the
// individual games, plus the registry of games shown in the lobby.
//
// A Game is a long-lived singleton created once at server start. It owns any
// state shared between players (rooms, matchmaking, the game world). Each
// time a player launches it from the lobby, Join is called to create a Bubble
// Tea model for that player's session. When the player is done, the model
// returns the Exit command and the platform takes them back to the lobby. If
// the model implements Leaver, it is told when the player leaves.
package games

import (
	tea "github.com/charmbracelet/bubbletea"

	"termcade/internal/ui/theme"
)

// Mode says whether a game is played alone or with others.
type Mode string

const (
	Solo        Mode = "Solo"
	Multiplayer Mode = "Multiplayer"
)

// Kind is the broad style of play.
type Kind string

const (
	Realtime  Kind = "Realtime"
	TurnBased Kind = "Turn-based"
	Party     Kind = "Party"
)

// Info describes a game for the lobby.
type Info struct {
	ID          string   // stable identifier, e.g. "tron"
	Name        string   // display name
	Mode        Mode     // solo or multiplayer
	Icon        string   // a single-cell glyph shown in the game list
	Tagline     string   // one short line
	Description string   // a few sentences
	Kind        Kind     // realtime, turn-based, ...
	Players     string   // e.g. "2–8"
	Controls    []string // pairs of key and action: {"←↑→↓", "steer", ...}
	Art         []string // small preview drawing, ideally ≤ 40×5
	Accent      []string // gradient stops for the art and highlights
}

// Player identifies who is joining a game.
type Player struct {
	SessionID string
	ID        int64 // 0 for guests
	Name      string
	Color     string // theme.PlayerColors key
	Guest     bool
}

// ScoreEntry is one line of a leaderboard.
type ScoreEntry struct {
	Name  string
	Color string
	Score int
}

// ScoreBook records results and reads leaderboards. A board is a
// leaderboard name such as "tetris" or "minesweeper:hard"; lowerIsBetter
// selects whether it ranks times or points. Guests' results only live for
// their session.
type ScoreBook interface {
	// Submit records a finished game and returns the player's best on the
	// board and whether this score set a new personal record.
	Submit(board string, lowerIsBetter bool, score int) (best int, record bool)
	// Best returns the player's best score, if any.
	Best(board string, lowerIsBetter bool) (int, bool)
	// Top returns the leaders of a board.
	Top(board string, lowerIsBetter bool, n int) []ScoreEntry
}

// Session is everything a game needs to build a player's model.
type Session struct {
	Player Player
	Theme  *theme.Theme
	Width  int
	Height int
	// Send delivers a message to this player's model from any goroutine,
	// e.g. a room's simulation loop broadcasting a new snapshot. It blocks
	// until the program accepts the message, so broadcast through a buffered
	// per-player queue (see internal/hub) rather than calling it while
	// holding a lock.
	Send func(tea.Msg)
	// Scores is the player's view of the leaderboards.
	Scores ScoreBook
}

// Leaver is implemented by game models that need to clean up when the player
// goes away. Leave is called exactly once, whether the player returned to
// the lobby or disconnected.
type Leaver interface {
	Leave()
}

// Game is implemented by every playable game.
type Game interface {
	Info() Info
	// Available reports whether the game can be played right now. Games that
	// are not yet implemented return false and are shown as "coming soon".
	Available() bool
	// Join creates the model for one player's session.
	Join(s Session) tea.Model
}

// ExitMsg tells the platform to return the player to the lobby.
type ExitMsg struct{}

// Exit is a command that returns the player to the lobby.
func Exit() tea.Msg { return ExitMsg{} }

// Registry is an ordered set of games.
type Registry struct {
	games []Game
	byID  map[string]Game
}

// NewRegistry returns a registry holding games in the given order.
func NewRegistry(gs ...Game) *Registry {
	r := &Registry{byID: make(map[string]Game)}
	for _, g := range gs {
		id := g.Info().ID
		if _, dup := r.byID[id]; dup {
			panic("games: duplicate game id " + id)
		}
		r.games = append(r.games, g)
		r.byID[id] = g
	}
	return r
}

// All returns the games in display order.
func (r *Registry) All() []Game { return r.games }

// Get returns the game with the given id.
func (r *Registry) Get(id string) (Game, bool) {
	g, ok := r.byID[id]
	return g, ok
}
