package games

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Placeholder is a game that is announced in the lobby but not playable yet.
type Placeholder struct{ info Info }

// ComingSoon announces a game that isn't built yet.
func ComingSoon(info Info) Game { return Placeholder{info} }

// Info implements Game.
func (p Placeholder) Info() Info { return p.info }

// Available implements Game.
func (Placeholder) Available() bool { return false }

// Join implements Game. It is never called for unavailable games.
func (Placeholder) Join(Session) tea.Model { return nil }
